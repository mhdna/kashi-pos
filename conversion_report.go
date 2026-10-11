package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	db "kashi-pos/db/sqlc"
)

// The Conversion page: how likely someone who walks in is to buy, from the
// nav-drawer people-visits counter (visitor_counts) and the day's sale
// invoices. A day's buy rate is its sales divided by its visitors; its
// money per visitor is that rate times the average invoice.
//
// Admins pick any range. A manager opens it with the manager password (the
// attendance approver's, see verifyManagerPassword) and always gets the
// current month so far, whatever range was asked for.

// ConversionStats is one span of days added up. Only days with a visitor
// count go into it: a day nobody counted has sales but no visitors, and
// would make the rate meaningless.
type ConversionStats struct {
	Days     int   `json:"days"`
	Visitors int64 `json:"visitors"`
	Invoices int64 `json:"invoices"`
	Revenue  int64 `json:"revenue"`
	// Invoices per visitor, 0..1 (above 1 when the counter missed people).
	BuyRate float64 `json:"buyRate"`
	// Cents, in the primary currency like invoices.grand_total.
	AvgInvoice        int64 `json:"avgInvoice"`
	RevenuePerVisitor int64 `json:"revenuePerVisitor"`
}

type ConversionGroup struct {
	Label string `json:"label"`
	ConversionStats
}

type ConversionDay struct {
	Day      string `json:"day"`
	Weekday  string `json:"weekday"`
	Visitors int64  `json:"visitors"`
	Invoices int64  `json:"invoices"`
	Revenue  int64  `json:"revenue"`
}

type ConversionReport struct {
	From string `json:"from"`
	To   string `json:"to"`
	// True for a manager: the range is the current month, not a choice.
	MonthOnly bool            `json:"monthOnly"`
	Overall   ConversionStats `json:"overall"`
	// Days with sales but no visitor count, left out of every figure.
	UncountedDays int `json:"uncountedDays"`
	// Counted days split by how busy they were: quiet, normal, busy thirds.
	ByTraffic []ConversionGroup `json:"byTraffic"`
	ByWeekday []ConversionGroup `json:"byWeekday"`
	Days      []ConversionDay   `json:"days"`
}

// GetConversionReport builds the Conversion page for from..to
// ("YYYY-MM-DD", inclusive). An admin's range is used as given; anyone else
// must pass the manager password and gets the current month to date.
func (a *App) GetConversionReport(from, to, managerPassword string) (*ConversionReport, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	monthOnly := !a.currentUserIsAdmin()
	if monthOnly {
		if err := a.verifyManagerPassword(managerPassword); err != nil {
			return nil, err
		}
		now := time.Now()
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		to = now.Format("2006-01-02")
	}

	fromDate, err := time.Parse("2006-01-02", strings.TrimSpace(from))
	if err != nil {
		return nil, fmt.Errorf("invalid from date %q; use YYYY-MM-DD", from)
	}
	toDate, err := time.Parse("2006-01-02", strings.TrimSpace(to))
	if err != nil {
		return nil, fmt.Errorf("invalid to date %q; use YYYY-MM-DD", to)
	}
	if fromDate.After(toDate) {
		return nil, fmt.Errorf("from date must not be after to date")
	}
	reportFrom, reportTo := fromDate.Format("2006-01-02"), toDate.Format("2006-01-02")

	visitors, err := a.store.ListVisitorCountsInRange(a.ctx, db.ListVisitorCountsInRangeParams{
		ReportFrom: reportFrom, ReportTo: reportTo,
	})
	if err != nil {
		return nil, err
	}
	sales, err := a.store.ListDailySalesTotals(a.ctx, db.ListDailySalesTotalsParams{
		ReportFrom: reportFrom, ReportTo: reportTo,
	})
	if err != nil {
		return nil, err
	}

	report := buildConversionReport(fromDate, toDate, visitors, sales)
	report.MonthOnly = monthOnly
	return report, nil
}

// buildConversionReport adds up the per-day rows. Every calendar day in the
// range is listed; one with neither visitors nor sales stays all zeros.
func buildConversionReport(fromDate, toDate time.Time, visitors []db.ListVisitorCountsInRangeRow, sales []db.ListDailySalesTotalsRow) *ConversionReport {
	visitorsByDay := make(map[string]int64, len(visitors))
	for _, v := range visitors {
		visitorsByDay[v.Day] = v.Count
	}
	salesByDay := make(map[string]db.ListDailySalesTotalsRow, len(sales))
	for _, s := range sales {
		salesByDay[s.Day] = s
	}

	report := &ConversionReport{
		From: fromDate.Format("2006-01-02"),
		To:   toDate.Format("2006-01-02"),
		Days: []ConversionDay{},
	}
	var counted []ConversionDay
	for d := fromDate; !d.After(toDate); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		day := ConversionDay{
			Day:      key,
			Weekday:  d.Weekday().String(),
			Visitors: visitorsByDay[key],
			Invoices: salesByDay[key].InvoiceCount,
			Revenue:  salesByDay[key].Revenue,
		}
		report.Days = append(report.Days, day)
		switch {
		case day.Visitors > 0:
			counted = append(counted, day)
		case day.Invoices > 0:
			report.UncountedDays++
		}
	}

	report.Overall = sumConversion(counted)
	report.ByTraffic = trafficGroups(counted)
	report.ByWeekday = weekdayGroups(counted)
	return report
}

func sumConversion(days []ConversionDay) ConversionStats {
	stats := ConversionStats{Days: len(days)}
	for _, d := range days {
		stats.Visitors += d.Visitors
		stats.Invoices += d.Invoices
		stats.Revenue += d.Revenue
	}
	if stats.Visitors > 0 {
		stats.BuyRate = float64(stats.Invoices) / float64(stats.Visitors)
		stats.RevenuePerVisitor = stats.Revenue / stats.Visitors
	}
	if stats.Invoices > 0 {
		stats.AvgInvoice = stats.Revenue / stats.Invoices
	}
	return stats
}

// trafficGroups splits the counted days, quietest first, into thirds, so
// the page can show whether a busy day converts better or worse than a
// quiet one. Fewer than three counted days can't be split.
func trafficGroups(days []ConversionDay) []ConversionGroup {
	if len(days) < 3 {
		return []ConversionGroup{}
	}
	sorted := append([]ConversionDay(nil), days...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Visitors < sorted[j].Visitors })

	names := []string{"Quiet days", "Normal days", "Busy days"}
	groups := make([]ConversionGroup, 0, len(names))
	for i, name := range names {
		part := sorted[i*len(sorted)/3 : (i+1)*len(sorted)/3]
		low, high := part[0].Visitors, part[len(part)-1].Visitors
		label := fmt.Sprintf("%s (%d–%d visitors)", name, low, high)
		if low == high {
			label = fmt.Sprintf("%s (%d visitors)", name, low)
		}
		groups = append(groups, ConversionGroup{Label: label, ConversionStats: sumConversion(part)})
	}
	return groups
}

// weekdayGroups adds up the counted days by weekday, Monday first, leaving
// out weekdays with no counted day.
func weekdayGroups(days []ConversionDay) []ConversionGroup {
	byName := map[string][]ConversionDay{}
	for _, d := range days {
		byName[d.Weekday] = append(byName[d.Weekday], d)
	}
	order := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday}
	groups := []ConversionGroup{}
	for _, wd := range order {
		if part := byName[wd.String()]; len(part) > 0 {
			groups = append(groups, ConversionGroup{Label: wd.String(), ConversionStats: sumConversion(part)})
		}
	}
	return groups
}
