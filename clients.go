package main

import (
	"fmt"

	db "kashi-pos/db/sqlc"
)

func (a *App) GetClientByPhone(phone string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	client, err := a.store.GetClientByPhone(a.ctx, phone)
	if err != nil {
		return nil, fmt.Errorf("client not found")
	}
	return map[string]any{
		"id":                 client.ID,
		"name":               client.Name,
		"phone":              client.Phone,
		"totalLoyaltyPoints": client.TotalLoyaltyPoints,
		"validLoyaltyPoints": client.ValidLoyaltyPoints,
	}, nil
}

func (a *App) CreateClient(name, phone string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	client, err := a.store.CreateClient(a.ctx, db.CreateClientParams{
		Name:               name,
		Phone:              phone,
		TotalLoyaltyPoints: 0,
		ValidLoyaltyPoints: 0,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":                 client.ID,
		"name":               client.Name,
		"phone":              client.Phone,
		"totalLoyaltyPoints": client.TotalLoyaltyPoints,
		"validLoyaltyPoints": client.ValidLoyaltyPoints,
	}, nil
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
	return map[string]any{
		"id":                 updated.ID,
		"name":               updated.Name,
		"phone":              updated.Phone,
		"totalLoyaltyPoints": updated.TotalLoyaltyPoints,
		"validLoyaltyPoints": updated.ValidLoyaltyPoints,
	}, nil
}
