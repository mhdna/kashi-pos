export function paymentColor(name: unknown): string {
  const value = String(name ?? "").toLowerCase();
  if (value.includes("visa")) return "purple-darken-2";
  if (value.includes("whish")) return "red-darken-2";
  if (value.includes("lbp") || value.includes("lebanese")) return "blue-darken-2";
  return "green-darken-2";
}

export function paymentTone(name: unknown): "green" | "purple" | "blue" | "red" {
  const color = paymentColor(name);
  if (color.startsWith("purple")) return "purple";
  if (color.startsWith("blue")) return "blue";
  if (color.startsWith("red")) return "red";
  return "green";
}

// The theme only has CSS custom properties for its own role colors
// (primary, surface, error, ...) - not for arbitrary Material color
// names like "green-darken-2", so `var(--k-theme-green-darken-2)` never
// resolves to anything. These are the actual Material Design hex values for
// the tokens paymentColor() returns, used directly instead.
const DEFAULT_ACCENT_HEX = "#617557";
const MATERIAL_COLOR_HEX: Record<string, string> = {
  "purple-darken-2": "#7B1FA2",
  "red-darken-2": "#D32F2F",
  "blue-darken-2": "#1976D2",
  "green-darken-2": DEFAULT_ACCENT_HEX,
};

// Resolves the accent color a payment method's card/button should use: an
// admin-picked hex color when set on the cashbox account, otherwise the
// existing name-based heuristic above.
export function paymentAccentColor(method: { name?: unknown; color?: unknown }): string {
  const custom = String(method.color ?? "").trim();
  if (custom) return custom;
  return MATERIAL_COLOR_HEX[paymentColor(method.name)] ?? DEFAULT_ACCENT_HEX;
}
