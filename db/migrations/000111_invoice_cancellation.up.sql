-- A sale rung up by mistake (a $3 item keyed in at $3000) is cancelled, not
-- deleted: CancelInvoice checks out a full return of it, and cancelled_at is
-- stamped on both the sale and that return. Sales statistics skip both;
-- the shift's cash and stock keep them, since the money and goods really did
-- go out and come back. kashi gets the same mark (see the cancellation flag
-- in branchInvoicePayload).
ALTER TABLE invoices ADD COLUMN cancelled_at text;
