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
		result[i] = map[string]any{"id": couponType.ID, "name": couponType.Name, "valueCents": couponType.ValueCents, "currencyCode": couponType.CurrencyCode, "isActive": couponType.IsActive != 0, "referenceOnly": couponType.ReferenceOnly != 0, "calculateOnCheckout": couponType.CalculateOnCheckout != 0}
	}
	return result, nil
}

func (a *App) ListCoupons() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	coupons, err := a.store.ListCoupons(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(coupons))
	for i, coupon := range coupons {
		result[i] = map[string]any{"code": coupon.Code, "typeId": coupon.TypeID.Int64, "valueCents": coupon.ValueCents.Int64, "status": coupon.Status, "clientId": coupon.ClientID, "redeemedInvoiceId": coupon.RedeemedInvoiceID.Int64}
	}
	return result, nil
}

func (a *App) DeleteCoupon(code string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return fmt.Errorf("coupon code is required")
	}
	coupon, err := a.store.GetCouponByCode(a.ctx, code)
	if err != nil {
		return fmt.Errorf("coupon not found")
	}
	payments, err := a.store.CountInvoiceCouponPaymentsByCode(a.ctx, coupon.Code)
	if err != nil {
		return err
	}
	if payments > 0 || coupon.RedeemedInvoiceID.Valid {
		return fmt.Errorf("redeemed coupons cannot be deleted")
	}
	return a.store.DeleteCoupon(a.ctx, coupon.Code)
}

func (a *App) DeleteCouponType(id int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if id <= 0 {
		return fmt.Errorf("coupon type is required")
	}
	if _, err := a.store.GetCouponTypeByID(a.ctx, id); err != nil {
		return fmt.Errorf("coupon type not found")
	}
	coupons, err := a.store.CountCouponsByTypeID(a.ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		return err
	}
	payments, err := a.store.CountInvoiceCouponPaymentsByTypeID(a.ctx, id)
	if err != nil {
		return err
	}
	if coupons > 0 || payments > 0 {
		return fmt.Errorf("coupon types used by coupons or invoices cannot be deleted")
	}
	return a.store.DeleteCouponType(a.ctx, id)
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

func (a *App) CreateCouponType(name, currencyCode string, valueCents int64, referenceOnly, calculateOnCheckout bool) (map[string]any, error) {
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
	calculateOnCheckoutValue := int64(0)
	if calculateOnCheckout {
		calculateOnCheckoutValue = 1
	}
	couponType, err := a.store.CreateCouponType(a.ctx, db.CreateCouponTypeParams{Name: name, ValueCents: valueCents, CurrencyCode: currencyCode, IsActive: 1, ReferenceOnly: referenceOnlyValue, CalculateOnCheckout: calculateOnCheckoutValue})
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": couponType.ID, "name": couponType.Name, "valueCents": couponType.ValueCents, "currencyCode": couponType.CurrencyCode, "isActive": true, "referenceOnly": couponType.ReferenceOnly != 0, "calculateOnCheckout": couponType.CalculateOnCheckout != 0}, nil
}

func (a *App) UpdateCouponType(id int64, name, currencyCode string, valueCents int64, isActive, referenceOnly, calculateOnCheckout bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	name = strings.TrimSpace(name)
	currencyCode = strings.ToUpper(strings.TrimSpace(currencyCode))
	if id <= 0 || name == "" || valueCents <= 0 {
		return fmt.Errorf("coupon type name and positive value are required")
	}
	if currencyCode == "" {
		currencyCode = "USD"
	}
	active := int64(0)
	if isActive {
		active = 1
	}
	reference := int64(0)
	if referenceOnly {
		reference = 1
	}
	calculateOnCheckoutValue := int64(0)
	if calculateOnCheckout {
		calculateOnCheckoutValue = 1
	}
	return a.store.UpdateCouponType(a.ctx, db.UpdateCouponTypeParams{
		Name: name, ValueCents: valueCents, CurrencyCode: currencyCode,
		IsActive: active, ReferenceOnly: reference, CalculateOnCheckout: calculateOnCheckoutValue, ID: id,
	})
}
