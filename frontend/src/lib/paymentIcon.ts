// The icon for a payment account row (the shift income card): what the money
// is, not a wallet for everything. Matched on the account's name, like
// paymentColor, so a new account named "Visa 2" or "Cash LBP" gets the right
// one without a setting.

export interface PaymentIconSource {
  name?: unknown;
  isOnAccount?: boolean;
  isOnCoupon?: boolean;
}

export function paymentIcon(account: PaymentIconSource): string {
  if (account.isOnAccount) return "mdi-account-clock";
  if (account.isOnCoupon) return "mdi-ticket-percent-outline";
  const name = String(account.name ?? "").toLowerCase();
  if (/coupon/.test(name)) return "mdi-ticket-percent-outline";
  if (/visa|master|card|credit|debit|pos\b/.test(name)) return "mdi-credit-card-outline";
  if (/whish|wallet|omt|mobile|phone/.test(name)) return "mdi-cellphone";
  if (/bank|transfer|wire|cheque|check/.test(name)) return "mdi-bank";
  // Lebanese pounds: the 100-note icon; dollars and plain "cash": banknote.
  if (/lbp|lebanese|\bll\b|ل\.ل|lira|pound/.test(name)) return "mdi-cash-100";
  if (/cash|\$|usd|dollar/.test(name)) return "mdi-cash";
  return "mdi-wallet";
}
