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
