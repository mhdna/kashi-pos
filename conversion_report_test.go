package main

import (
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestBuildConversionReport_RatesSkipUncountedDays(t *testing.T) {
	visitors := []db.ListVisitorCountsInRangeRow{
		{Day: "2026-10-05", Count: 10}, // Monday
		{Day: "2026-10-06", Count: 40},
		{Day: "2026-10-07", Count: 20},
	}
	sales := []db.ListDailySalesTotalsRow{
		{Day: "2026-10-05", InvoiceCount: 5, Revenue: 5000},
		{Day: "2026-10-06", InvoiceCount: 8, Revenue: 16000},
		{Day: "2026-10-07", InvoiceCount: 4, Revenue: 4000},
		// Sales on a day nobody counted visitors: left out of the rates.
		{Day: "2026-10-08", InvoiceCount: 9, Revenue: 90000},
	}
	r := buildConversionReport(mustDate(t, "2026-10-05"), mustDate(t, "2026-10-09"), visitors, sales)

	if len(r.Days) != 5 {
		t.Fatalf("days = %d, want every calendar day (5)", len(r.Days))
	}
	if r.UncountedDays != 1 {
		t.Fatalf("uncounted days = %d, want 1", r.UncountedDays)
	}
	o := r.Overall
	if o.Days != 3 || o.Visitors != 70 || o.Invoices != 17 || o.Revenue != 25000 {
		t.Fatalf("overall = %+v", o)
	}
	if got, want := o.BuyRate, 17.0/70.0; got != want {
		t.Fatalf("buy rate = %v, want %v", got, want)
	}
	if o.AvgInvoice != 25000/17 || o.RevenuePerVisitor != 25000/70 {
		t.Fatalf("avg invoice %d, per visitor %d", o.AvgInvoice, o.RevenuePerVisitor)
	}

	// Quietest third first: 10, then 20, then 40 visitors.
	if len(r.ByTraffic) != 3 {
		t.Fatalf("traffic groups = %d, want 3", len(r.ByTraffic))
	}
	if r.ByTraffic[0].Label != "Quiet days (10 visitors)" || r.ByTraffic[0].BuyRate != 0.5 {
		t.Fatalf("quiet group = %+v", r.ByTraffic[0])
	}
	if r.ByTraffic[2].Label != "Busy days (40 visitors)" || r.ByTraffic[2].BuyRate != 0.2 {
		t.Fatalf("busy group = %+v", r.ByTraffic[2])
	}

	if len(r.ByWeekday) != 3 || r.ByWeekday[0].Label != "Monday" {
		t.Fatalf("weekdays = %+v", r.ByWeekday)
	}
}

func TestBuildConversionReport_TooFewDaysForTraffic(t *testing.T) {
	r := buildConversionReport(mustDate(t, "2026-10-05"), mustDate(t, "2026-10-05"),
		[]db.ListVisitorCountsInRangeRow{{Day: "2026-10-05", Count: 3}}, nil)
	if len(r.ByTraffic) != 0 {
		t.Fatalf("traffic groups = %+v, want none for one day", r.ByTraffic)
	}
	if r.Overall.BuyRate != 0 || r.Overall.AvgInvoice != 0 {
		t.Fatalf("no sales should give zero rates: %+v", r.Overall)
	}
}

// A manager needs the manager password and always gets this month so far;
// an admin gets the range asked for.
func TestGetConversionReport_Access(t *testing.T) {
	app := newVisitorTestApp(t)
	if err := app.UpdateAttendanceApproverPassword("1234"); err != nil {
		t.Fatal(err)
	}

	app.currentRole = "cashier"
	if _, err := app.GetConversionReport("2026-01-01", "2026-01-31", "wrong"); err == nil {
		t.Fatal("wrong manager password was accepted")
	}
	r, err := app.GetConversionReport("2026-01-01", "2026-01-31", "1234")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if !r.MonthOnly || r.From != now.Format("2006-01")+"-01" || r.To != now.Format("2006-01-02") {
		t.Fatalf("manager range = %s..%s monthOnly=%v, want this month to date", r.From, r.To, r.MonthOnly)
	}

	app.currentRole = "admin"
	r, err = app.GetConversionReport("2026-01-01", "2026-01-31", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.MonthOnly || r.From != "2026-01-01" || r.To != "2026-01-31" || len(r.Days) != 31 {
		t.Fatalf("admin range = %s..%s monthOnly=%v days=%d", r.From, r.To, r.MonthOnly, len(r.Days))
	}
}
