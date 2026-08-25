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
		settings, err := q.GetSettings(a.ctx)
		if err != nil {
			return err
		}
		return queueBranchClientSync(a.ctx, q, settings.CentralApiUrl, client.ID, client.Name, client.Phone)
	})
	if err != nil {
		return nil, err
	}
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
		return queueBranchClientSync(a.ctx, q, settings.CentralApiUrl, updated.ID, updated.Name, updated.Phone)
	})
	if err != nil {
		return nil, err
	}
	return clientResponse(updated), nil
}

func (a *App) DeleteClient(clientID int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if clientID <= 0 {
		return fmt.Errorf("invalid client")
	}
	if _, err := a.store.GetClientByID(a.ctx, clientID); err != nil {
		return fmt.Errorf("client not found")
	}
	return a.store.DeleteClient(a.ctx, clientID)
}

func normalizeClientPhone(phone string) (string, error) {
	phone = strings.TrimSpace(phone)
	if phone == "00000000" {
		return phone, nil
	}
	parsed, err := phonenumbers.Parse(phone, "LB")
	if err != nil || !phonenumbers.IsValidNumber(parsed) {
		return "", fmt.Errorf("invalid phone number")
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
	}
}
