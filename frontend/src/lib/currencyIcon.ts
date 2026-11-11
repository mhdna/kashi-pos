// The icon beside a currency code (swap balance, ...). MDI has signs for the
// big currencies; any other (Lebanese lira, ...) shows banknotes. Every icon
// here is registered in icons.ts.
const SIGNS: Record<string, string> = {
  USD: "mdi-currency-usd",
  EUR: "mdi-currency-eur",
  GBP: "mdi-currency-gbp",
};

export function currencyIcon(code: unknown): string {
  return SIGNS[String(code ?? "").trim().toUpperCase()] ?? "mdi-cash";
}
