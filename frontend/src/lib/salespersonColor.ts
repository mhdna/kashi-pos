// Same palette kashi picks a random salesperson color from (migration
// 000112_salesperson_color in the kashi repo) -- all readable under white
// initials.
const PALETTE = [
  "#E53935", "#D81B60", "#8E24AA", "#5E35B1", "#3949AB", "#1E88E5",
  "#00838F", "#00897B", "#43A047", "#F4511E", "#6D4C41", "#546E7A",
];

/**
 * Avatar background for a salesperson: the color set in kashi, or for a
 * till-only salesperson (standalone mode, or not synced yet) a stable pick
 * from the palette based on the name, so it doesn't change between renders.
 */
export function salespersonColor(person: { color?: string | null; fullName?: string | null } | null | undefined) {
  const color = person?.color?.trim();
  if (color) return color;
  const name = (person?.fullName ?? "").trim().toLowerCase();
  let hash = 0;
  for (const ch of name) hash = (hash * 31 + ch.codePointAt(0)!) >>> 0;
  return PALETTE[hash % PALETTE.length];
}
