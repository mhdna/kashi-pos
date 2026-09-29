package main

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	db "kashi-pos/db/sqlc"
)

type transferRequestItemInput struct {
	Barcode  string
	Quantity int64
}

// CreateTransferRequest stages a proposed transfer out of this branch's own
// store inventory and, if this install is configured to sync, queues it for
// push to kashi in the same transaction as the write itself -- same
// same-transaction-as-the-write reasoning as CreateExpense. toBranchCode is
// another branch's code, as last seen in the branches sync feed (see
// ListSyncedBranches). The transfer sits centrally in 'requested' until a
// kashi admin approves or rejects it; this till only finds out via the
// transfer_requests down-sync (see applySyncedTransferRequest) or, if
// connected, the WS nudge that follows kashi's decision.
//
// A photo, if any, is never pushed by this till -- it's the same
// QR-scan-and-upload flow as a close-shift receipt or an expense photo
// (see CheckTransferUploadStatus and emitTransferUploadReady): once this
// call's sync round-trip lands, kashi hands back a one-time upload token
// the frontend turns into a QR code for a phone to scan.
func (a *App) CreateTransferRequest(toBranchCode, note string, items []transferRequestItemInput) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if toBranchCode == "" {
		return nil, fmt.Errorf("destination branch is required")
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("at least one item is required")
	}

	var transfer db.TransferRequest
	err := a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		settings, err := q.GetSettings(a.ctx)
		if err != nil {
			return fmt.Errorf("load settings: %w", err)
		}

		clientRef := uuid.NewString()
		transfer, err = q.CreateTransferRequest(a.ctx, db.CreateTransferRequestParams{
			ClientRef:    clientRef,
			ToBranchCode: toBranchCode,
			Note:         note,
		})
		if err != nil {
			return fmt.Errorf("create transfer request: %w", err)
		}

		payloadItems := make([]branchTransferRequestItemPayload, 0, len(items))
		for _, item := range items {
			if item.Barcode == "" || item.Quantity == 0 {
				continue
			}
			product, err := q.GetProductByBarcode(a.ctx, item.Barcode)
			if err != nil {
				return fmt.Errorf("product %q: %w", item.Barcode, err)
			}
			if _, err := q.CreateTransferRequestItem(a.ctx, db.CreateTransferRequestItemParams{
				TransferRequestID: transfer.ID,
				Barcode:           item.Barcode,
				ProductUid:        product.Uid.String,
				Quantity:          item.Quantity,
			}); err != nil {
				return fmt.Errorf("create transfer request item: %w", err)
			}
			payloadItems = append(payloadItems, branchTransferRequestItemPayload{
				Barcode:    item.Barcode,
				ProductUID: product.Uid.String,
				Quantity:   item.Quantity,
			})
		}
		if len(payloadItems) == 0 {
			return fmt.Errorf("at least one item is required")
		}

		if settings.CentralApiUrl == "" {
			return nil
		}
		return queueBranchTransferRequestSync(a.ctx, q, settings.CentralApiUrl, branchTransferRequestPayload{
			ClientRef:   clientRef,
			ToBranchRef: toBranchCode,
			Note:        note,
			Items:       payloadItems,
		})
	})
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"id":        transfer.ID,
		"clientRef": transfer.ClientRef,
		"status":    transfer.Status,
	}, nil
}

// ListTransferRequests feeds this till's own "my transfer requests" screen
// -- outgoing proposals only, newest first, whatever status kashi last
// reported. See ListIncomingTransferRequests for transfers addressed to
// this branch instead.
func (a *App) ListTransferRequests() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := a.store.ListOutgoingTransferRequests(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(rows))
	for i, r := range rows {
		result[i] = map[string]any{
			"id": r.ID, "clientRef": r.ClientRef, "toBranchCode": r.ToBranchCode,
			"note": r.Note, "status": r.Status, "createdAt": r.CreatedAt,
		}
	}
	return result, nil
}

// ListIncomingTransferRequests feeds this branch's "incoming" screen --
// transfers other branches have sent here, whoever sent them, with their
// items so the till can show what to count and confirm.
func (a *App) ListIncomingTransferRequests() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := a.store.ListIncomingTransferRequests(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(rows))
	for i, r := range rows {
		items, err := a.store.ListTransferRequestItems(a.ctx, r.ID)
		if err != nil {
			return nil, err
		}
		itemRows := make([]map[string]any, len(items))
		for j, it := range items {
			row := map[string]any{
				"id": it.ID, "barcode": it.Barcode, "productUid": it.ProductUid, "quantity": it.Quantity,
			}
			if it.ReceivedQuantity.Valid {
				row["receivedQuantity"] = it.ReceivedQuantity.Int64
			}
			itemRows[j] = row
		}
		result[i] = map[string]any{
			"id": r.CentralID.Int64, "senderBranchName": r.SenderBranchName,
			"note": r.Note, "status": r.Status, "createdAt": r.CreatedAt, "items": itemRows,
		}
	}
	return result, nil
}

type confirmTransferItemInput struct {
	Barcode          string
	ProductUID       string
	ReceivedQuantity int64
}

// ConfirmTransferRequest is this branch reporting what it actually
// counted for an incoming transfer -- it records the counted quantities
// locally (so applySyncedIncomingTransfer's replace-on-sync doesn't lose
// them, see its own comment) and queues the confirmation for push to
// kashi via the same sync_outbox worker every other entity uses.
func (a *App) ConfirmTransferRequest(centralID int64, items []confirmTransferItemInput, note string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		transfer, err := q.GetTransferRequestByCentralID(a.ctx, sql.NullInt64{Int64: centralID, Valid: true})
		if err != nil {
			return fmt.Errorf("load transfer request: %w", err)
		}

		localItems, err := q.ListTransferRequestItems(a.ctx, transfer.ID)
		if err != nil {
			return fmt.Errorf("load transfer request items: %w", err)
		}
		byBarcode := make(map[string]int64, len(localItems))
		for _, it := range localItems {
			byBarcode[it.Barcode] = it.ID
		}

		settings, err := q.GetSettings(a.ctx)
		if err != nil {
			return fmt.Errorf("load settings: %w", err)
		}

		payloadItems := make([]branchTransferConfirmItemPayload, 0, len(items))
		for _, item := range items {
			id, ok := byBarcode[item.Barcode]
			if !ok {
				continue
			}
			if err := q.SetTransferRequestItemReceivedQuantity(a.ctx, db.SetTransferRequestItemReceivedQuantityParams{
				ID:               id,
				ReceivedQuantity: sql.NullInt64{Int64: item.ReceivedQuantity, Valid: true},
			}); err != nil {
				return fmt.Errorf("set counted quantity: %w", err)
			}
			payloadItems = append(payloadItems, branchTransferConfirmItemPayload{
				Barcode: item.Barcode, ProductUID: item.ProductUID, ReceivedQuantity: item.ReceivedQuantity,
			})
		}

		if settings.CentralApiUrl == "" {
			return nil
		}
		return queueBranchTransferConfirmSync(a.ctx, q, settings.CentralApiUrl, centralID, branchTransferConfirmPayload{
			Items: payloadItems,
			Note:  note,
		})
	})
}

// ListDestinationBranches feeds the destination picker on the transfer
// request screen -- every other active branch this till has pulled down
// (see applySyncedBranch).
func (a *App) ListDestinationBranches() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := a.store.ListSyncedBranches(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(rows))
	for i, r := range rows {
		result[i] = map[string]any{"code": r.Code, "name": r.Name}
	}
	return result, nil
}

// CheckTransferUploadStatus is the poll-based companion to the
// "transfer_upload_ready:<clientRef>" event (see emitTransferUploadReady):
// once that event hands the frontend a QR link, this is what the open QR
// dialog calls every few seconds to find out a phone actually finished
// uploading the photo. Mirrors CheckExpenseUploadStatus.
func (a *App) CheckTransferUploadStatus(clientRef string) (bool, error) {
	if a.store == nil {
		return false, fmt.Errorf("database not initialized")
	}

	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return false, fmt.Errorf("load settings: %w", err)
	}
	if settings.CentralApiUrl == "" {
		return false, errCentralSyncDisabled
	}

	client := newCentralClient(settings.CentralApiUrl, settings.BranchCode, settings.ApiKey)
	return client.TransferUploadStatus(a.ctx, clientRef)
}
