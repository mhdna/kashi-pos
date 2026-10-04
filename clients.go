package main

import (
	"fmt"
	"strings"

	"github.com/nyaruka/phonenumbers"
	db "kashi-pos/db/sqlc"
)

func (a *App) GetClientByPhone(phone string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	phone, err := normalizeClientPhone(phone)
	if err != nil {
		return nil, err
	}
	client, err := a.store.GetClientByPhone(a.ctx, phone)
	if err != nil {
		return nil, fmt.Errorf("client not found")
	}
	return clientResponse(client), nil
}

func (a *App) SearchClients(search string, selectedID int64) ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	search = strings.TrimSpace(search)
	if len(search) > 80 {
		search = search[:80]
	}

	limit := int64(25)
	if selectedID > 0 {
		limit--
	}
	clients, err := a.store.SearchClients(a.ctx, db.SearchClientsParams{
		Search: search, ResultLimit: limit,
	})
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, 0, 25)
	selectedIncluded := false
	for _, client := range clients {
		if isWalkInClient(client) {
			continue
		}
		if client.IsActive == 0 && client.ID != selectedID {
			continue
		}
		if client.ID == selectedID {
			selectedIncluded = true
		}
		result = append(result, clientResponse(client))
	}
	if selectedID > 0 && !selectedIncluded {
		if selected, selectedErr := a.store.GetClientByID(a.ctx, selectedID); selectedErr == nil {
			result = append([]map[string]any{clientResponse(selected)}, result...)
		}
	}
	return result, nil
}

func (a *App) CreateClient(name, phone string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	phone, err := normalizeClientPhone(phone)
	if err != nil {
		return nil, err
	}
	var client db.Client
	err = a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		var err error
		client, err = q.CreateClient(a.ctx, db.CreateClientParams{
			Name:               name,
			Phone:              phone,
			TotalLoyaltyPoints: 0,
			ValidLoyaltyPoints: 0,
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Deliberately not queued for sync: kashi owns the shared client roster,
	// and a branch only pushes a client up once they've actually bought
	// something (see the checkout paths). A never-purchasing local client
	// stays local.
	return clientResponse(client), nil
}

func (a *App) UpdateClient(clientID int64, name, phone string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("client name is required")
	}
	normalizedPhone, err := normalizeClientPhone(phone)
	if err != nil {
		return nil, err
	}
	client, err := a.store.GetClientByID(a.ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("client not found")
	}
	if isWalkInClient(client) {
		return nil, fmt.Errorf("the walk-in customer cannot be edited")
	}
	var updated db.Client
	err = a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		var err error
		updated, err = q.UpdateClient(a.ctx, db.UpdateClientParams{
			Name:               name,
			Phone:              normalizedPhone,
			TotalLoyaltyPoints: client.TotalLoyaltyPoints,
			ValidLoyaltyPoints: client.ValidLoyaltyPoints,
			ID:                 clientID,
		})
		if err != nil {
			return err
		}
		settings, err := q.GetSettings(a.ctx)
		if err != nil {
			return err
		}
		// Only propagate the edit if this client is one we've pushed --
		// i.e. they have a purchase. Otherwise it's a local-only record.
		hasInvoice, err := q.ClientHasInvoice(a.ctx, clientID)
		if err != nil {
			return err
		}
		if hasInvoice == 0 {
			return nil
		}
		return queueBranchClientSync(a.ctx, q, settings.CentralApiUrl, updated)
	})
	if err != nil {
		return nil, err
	}
	a.kickSyncWorker()
	return clientResponse(updated), nil
}

// DeleteClient removes a client in standalone mode -- or deactivates it
// when any invoice, coupon or account entry refers to it -- and returns
// "deleted" or "deactivated". While connected to kashi, clients are
// deactivated centrally and this is refused (see standalone.go).
func (a *App) DeleteClient(clientID int64) (string, error) {
	if err := a.requireStandalone(); err != nil {
		return "", err
	}
	if clientID <= 0 {
		return "", fmt.Errorf("invalid client")
	}
	existing, err := a.store.GetClientByID(a.ctx, clientID)
	if err != nil {
		return "", fmt.Errorf("client not found")
	}
	if isWalkInClient(existing) {
		return "", fmt.Errorf("the walk-in customer cannot be deleted")
	}
	outcome := "deleted"
	err = a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		referenced, err := q.ClientIsReferenced(a.ctx, clientID)
		if err != nil {
			return err
		}
		if referenced != 0 {
			outcome = "deactivated"
			return q.SetClientActive(a.ctx, db.SetClientActiveParams{IsActive: 0, ID: clientID})
		}
		return q.DeleteClient(a.ctx, clientID)
	})
	if err != nil {
		return "", err
	}
	return outcome, nil
}

// walkInClientPhone is the sentinel phone of the seeded "Walk-in" pseudo-client
// that anonymous counter sales are booked against (see resolveClientID). It is
// not a real customer: it must never earn or redeem loyalty points, carry an
// account balance, or show up in the customer list.
const walkInClientPhone = "00000000"

func isWalkInPhone(phone string) bool { return phone == walkInClientPhone }

func isWalkInClient(c db.Client) bool { return isWalkInPhone(c.Phone) }

// lebaneseMobilePrefixes are the 8-digit mobile ranges (Alfa/Touch). The
// phonenumbers metadata lags behind here -- it rejects most of 78/79 -- so
// Lebanese numbers are checked against this list first.
var lebaneseMobilePrefixes = []string{"70", "71", "76", "78", "79", "81"}

// normalizeLebanesePhone recognises a Lebanese number typed with or without
// the leading 0, +961 or 00961, and returns it in E.164. Accepts mobiles
// (03 and the 8-digit ranges above) and landlines (01, 04-09). ok is false
// for anything else, including numbers with a non-Lebanese country code.
func normalizeLebanesePhone(phone string) (string, bool) {
	phone = strings.TrimSpace(phone)
	if strings.HasPrefix(phone, "+") && !strings.HasPrefix(phone, "+961") {
		return "", false
	}
	var digits strings.Builder
	for _, r := range phone {
		switch {
		case r >= '0' && r <= '9':
			digits.WriteRune(r)
		case r == '+' || r == ' ' || r == '-' || r == '(' || r == ')':
		default:
			return "", false
		}
	}
	n := digits.String()
	switch {
	case strings.HasPrefix(n, "00961"):
		n = n[5:]
	case strings.HasPrefix(n, "961") && len(n) >= 10:
		n = n[3:]
	}
	n = strings.TrimPrefix(n, "0")
	switch len(n) {
	case 8:
		for _, prefix := range lebaneseMobilePrefixes {
			if strings.HasPrefix(n, prefix) {
				return "+961" + n, true
			}
		}
	case 7:
		// 3xxxxxx is the old 03 mobile range; 1 and 4-9 are the regional
		// landline codes (01 Beirut, 07 South, ...).
		if strings.ContainsRune("13456789", rune(n[0])) {
			return "+961" + n, true
		}
	}
	return "", false
}

func normalizeClientPhone(phone string) (string, error) {
	phone = strings.TrimSpace(phone)
	if phone == walkInClientPhone {
		return phone, nil
	}
	if normalized, ok := normalizeLebanesePhone(phone); ok {
		return normalized, nil
	}
	parsed, err := phonenumbers.Parse(phone, "LB")
	if err != nil {
		return "", fmt.Errorf("invalid phone number")
	}
	// Numbers with no explicit country code (or an explicit +961) are
	// Lebanese, since "LB" is always the default region: give those a
	// Lebanon-specific message instead of the generic one.
	isLebanese := phonenumbers.GetRegionCodeForNumber(parsed) == "LB"
	if !phonenumbers.IsValidNumber(parsed) {
		if isLebanese {
			return "", fmt.Errorf("enter a valid Lebanese phone number")
		}
		return "", fmt.Errorf("invalid phone number")
	}
	// Only mobile numbers are accepted: clients are contacted over SMS for
	// loyalty/receipts, and a landline/voicemail/premium-rate number can't
	// receive those. This also rejects mistyped numbers that happen to be
	// "valid" for some other Lebanese line type (e.g. the 80xxxxxx
	// voicemail range or 9xxxxxxx premium-rate range).
	switch phonenumbers.GetNumberType(parsed) {
	case phonenumbers.MOBILE, phonenumbers.FIXED_LINE_OR_MOBILE:
	default:
		if isLebanese {
			return "", fmt.Errorf("enter a valid Lebanese phone number")
		}
		return "", fmt.Errorf("enter a valid mobile phone number")
	}
	return phonenumbers.Format(parsed, phonenumbers.E164), nil
}

func (a *App) AddClientLoyaltyPoints(clientID int64, points int64) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	client, err := a.store.GetClientByID(a.ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("client not found")
	}
	if isWalkInClient(client) {
		return nil, fmt.Errorf("the walk-in customer cannot hold loyalty points")
	}
	updated, err := a.store.UpdateClient(a.ctx, db.UpdateClientParams{
		Name:               client.Name,
		Phone:              client.Phone,
		TotalLoyaltyPoints: client.TotalLoyaltyPoints + points,
		ValidLoyaltyPoints: client.ValidLoyaltyPoints + points,
		ID:                 clientID,
	})
	if err != nil {
		return nil, err
	}
	return clientResponse(updated), nil
}

func clientResponse(client db.Client) map[string]any {
	return map[string]any{
		"id":                 client.ID,
		"name":               client.Name,
		"phone":              client.Phone,
		"totalLoyaltyPoints": client.TotalLoyaltyPoints,
		"validLoyaltyPoints": client.ValidLoyaltyPoints,
		"isActive":           client.IsActive != 0,
	}
}
