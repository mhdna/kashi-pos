// Whether a checkout prints its receipt. A sale prints whenever receipts are
// on; an exchange also needs its "Print receipt" tick, which starts from the
// "Print return receipts" setting each time an exchange is loaded and the
// cashier can flip for that one exchange. The backend applies the same rule
// (tx_exchange.go printsReturnReceipt); an unprinted one can be reprinted
// from Invoices.

export type CheckoutMode = "sale" | "exchange";

export function checkoutPrints(mode: CheckoutMode, receiptsOn: boolean, returnTick: boolean): boolean {
  if (!receiptsOn) return false;
  return mode === "sale" || returnTick;
}

/** The tick an exchange starts with: the setting. */
export function startingReturnTick(printReturnReceipts: boolean): boolean {
  return printReturnReceipts;
}

/** The snackbar after an exchange checks out. */
export function exchangeDoneMessage(invoiceCode: string, printed: boolean, receiptError?: string): string {
  if (receiptError) return `Invoice created; receipt failed — ${receiptError}`;
  return printed ? `Exchange completed — ${invoiceCode}` : `Exchange completed, no receipt printed — ${invoiceCode}`;
}
