// Whether a checkout prints its receipt. A sale prints whenever receipts are
// on; an exchange also needs this till's "Print return and exchange
// receipts", which is set from central (branch settings; it reaches the till
// with its display prefs). The backend applies the same rule (tx_exchange.go
// printsReturnReceipt); an unprinted one can be reprinted from Invoices or
// from central's Print on till.

export type CheckoutMode = "sale" | "exchange";

export function checkoutPrints(mode: CheckoutMode, receiptsOn: boolean, returnTick: boolean): boolean {
  if (!receiptsOn) return false;
  return mode === "sale" || returnTick;
}

/** The snackbar after an exchange checks out. */
export function exchangeDoneMessage(invoiceCode: string, printed: boolean, receiptError?: string): string {
  if (receiptError) return `Invoice created; receipt failed — ${receiptError}`;
  return printed ? `Exchange completed — ${invoiceCode}` : `Exchange completed, no receipt printed — ${invoiceCode}`;
}
