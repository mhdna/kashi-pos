package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	db "kashi-pos/db/sqlc"
)

func (a *App) ListCouponTypes() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	types, err := a.store.ListCouponTypes(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(types))
	for i, couponType := range types {
		result[i] = map[string]any{"id": couponType.ID, "name": couponType.Name, "valueCents": couponType.ValueCents, "currencyCode": couponType.CurrencyCode, "isActive": couponType.IsActive != 0}
	}
	return result, nil
}

func (a *App) CreateCoupon(code string, couponTypeID, clientID int64) (map[string]any, error) {
	code = strings.TrimSpace(code)
	if code == "" || couponTypeID <= 0 {
		return nil, fmt.Errorf("coupon code and type are required")
	}
	couponType, err := a.store.GetCouponTypeByID(a.ctx, couponTypeID)
	if err != nil {
		return nil, fmt.Errorf("coupon type: %w", err)
	}
	clientID, err = a.resolveClientID(clientID)
	if err != nil {
		return nil, err
	}
	coupon, err := a.store.CreateValueCoupon(a.ctx, db.CreateValueCouponParams{
		Code: code, Reason: couponType.Name, ClientID: clientID,
		ValidUntil: time.Now().AddDate(10, 0, 0).Format("2006-01-02"), TypeID: sql.NullInt64{Int64: couponTypeID, Valid: true}, ValueCents: sql.NullInt64{Int64: couponType.ValueCents, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"code": coupon.Code, "typeId": coupon.TypeID.Int64, "valueCents": coupon.ValueCents.Int64, "status": coupon.Status}, nil
}

func (a *App) CreateCouponType(name, currencyCode string, valueCents int64, referenceOnly bool) (map[string]any, error) {
	name = strings.TrimSpace(name)
	currencyCode = strings.ToUpper(strings.TrimSpace(currencyCode))
	if name == "" || valueCents <= 0 {
		return nil, fmt.Errorf("coupon type name and positive value are required")
	}
	if currencyCode == "" {
		currencyCode = "USD"
	}
	referenceOnlyValue := int64(0)
	if referenceOnly {
		referenceOnlyValue = 1
	}
	couponType, err := a.store.CreateCouponType(a.ctx, db.CreateCouponTypeParams{Name: name, ValueCents: valueCents, CurrencyCode: currencyCode, IsActive: 1, ReferenceOnly: referenceOnlyValue})
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": couponType.ID, "name": couponType.Name, "valueCents": couponType.ValueCents, "currencyCode": couponType.CurrencyCode, "isActive": true, "referenceOnly": couponType.ReferenceOnly != 0}, nil
}
