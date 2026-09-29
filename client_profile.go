package main

import (
	"fmt"
	"strings"
)

// GetClientProfile returns all data needed to render the client profile page.
func (a *App) GetClientProfile(clientID int64) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if clientID <= 0 {
		return nil, fmt.Errorf("invalid client")
	}

	client, err := a.store.GetClientByID(a.ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("client not found")
	}

	stats, err := a.store.GetClientProfileStats(a.ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("load client stats: %w", err)
	}

	homeBranch, err := a.store.GetClientHomeBranch(a.ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("load home branch: %w", err)
	}

	familySpend, err := a.store.ListClientFamilySpend(a.ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("load category spend: %w", err)
	}

	paymentMix, err := a.store.ListClientPaymentMix(a.ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("load payment mix: %w", err)
	}

	recentInvoices, err := a.store.ListClientRecentInvoices(a.ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("load recent invoices: %w", err)
	}

	monthlySpend, err := a.store.ListClientMonthlySpend(a.ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("load monthly spend: %w", err)
	}

	// Build family spend list (family → [{kind, cents}])
	type kindEntry struct {
		Kind       string `json:"kind"`
		TotalCents int64  `json:"totalCents"`
	}
	type familyEntry struct {
		Family     string      `json:"family"`
		TotalCents int64       `json:"totalCents"`
		Kinds      []kindEntry `json:"kinds"`
	}
	familyMap := map[string]*familyEntry{}
	familyOrder := []string{}
	for _, row := range familySpend {
		if _, ok := familyMap[row.Family]; !ok {
			familyMap[row.Family] = &familyEntry{Family: row.Family}
			familyOrder = append(familyOrder, row.Family)
		}
		fe := familyMap[row.Family]
		fe.TotalCents += row.TotalCents
		fe.Kinds = append(fe.Kinds, kindEntry{Kind: row.Kind, TotalCents: row.TotalCents})
	}
	familyList := make([]any, 0, len(familyOrder))
	for _, f := range familyOrder {
		familyList = append(familyList, familyMap[f])
	}

	paymentMixList := make([]map[string]any, 0, len(paymentMix))
	for _, row := range paymentMix {
		paymentMixList = append(paymentMixList, map[string]any{
			"name":       row.AccountName,
			"totalCents": row.TotalCents,
		})
	}

	recentList := make([]map[string]any, 0, len(recentInvoices))
	for _, row := range recentInvoices {
		methods := []string{}
		for _, m := range strings.Split(row.PaymentMethods, ",") {
			m = strings.TrimSpace(m)
			if m != "" {
				methods = append(methods, m)
			}
		}
		recentList = append(recentList, map[string]any{
			"id": row.ID, "invoiceCode": row.InvoiceCode, "grandTotal": row.GrandTotal,
			"createdAt": row.CreatedAt, "branchName": row.BranchName,
			"itemCount": row.ItemCount, "paymentMethods": methods,
		})
	}

	monthlyList := make([]map[string]any, 0, len(monthlySpend))
	for _, row := range monthlySpend {
		monthlyList = append(monthlyList, map[string]any{
			"month": row.Month, "totalCents": row.TotalCents, "itemCount": row.ItemCount,
		})
	}

	pointsRedeemed := client.TotalLoyaltyPoints - client.ValidLoyaltyPoints
	if pointsRedeemed < 0 {
		pointsRedeemed = 0
	}
	firstPurchase := ""
	if stats.FirstPurchaseAt.Valid {
		firstPurchase = stats.FirstPurchaseAt.String
	}

	uid := ""
	if client.Uid.Valid {
		uid = client.Uid.String
	}

	return map[string]any{
		"client": map[string]any{
			"id": client.ID, "name": client.Name, "phone": client.Phone,
			"uid": uid, "createdAt": client.CreatedAt,
			"totalLoyaltyPoints": client.TotalLoyaltyPoints,
			"validLoyaltyPoints": client.ValidLoyaltyPoints,
		},
		"totalSpentCents":  stats.TotalSpentCents,
		"itemsBought":      stats.ItemsBought,
		"pointsRemaining":  client.ValidLoyaltyPoints,
		"pointsRedeemed":   pointsRedeemed,
		"homeBranch":       homeBranch,
		"firstPurchaseAt":  firstPurchase,
		"categorySpend":    familyList,
		"paymentMix":       paymentMixList,
		"recentInvoices":   recentList,
		"monthlySpend":     monthlyList,
	}, nil
}
