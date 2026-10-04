import { ref } from "vue";

export const isCompactBuild = import.meta.env.VITE_KASHI_COMPACT === "1";

const storedAnimations = localStorage.getItem("kashi-animations-enabled");
export const animationsEnabled = ref(
  storedAnimations === null ? !isCompactBuild : storedAnimations === "true",
);

export function setAnimationsEnabled(enabled: boolean) {
  animationsEnabled.value = enabled;
  localStorage.setItem("kashi-animations-enabled", String(enabled));
}

/** Currency code shown first on the calculator's total. Cycled by the
 * flip icon on the calculator itself, or set as a default from Settings. */
const storedTotalCurrency = localStorage.getItem("kashi-calculator-total-currency");
export const calculatorTotalCurrency = ref(storedTotalCurrency || "USD");

export function setCalculatorTotalCurrency(code: string) {
  calculatorTotalCurrency.value = code;
  localStorage.setItem("kashi-calculator-total-currency", code);
}

/** How many rows of cards the quick-access panel shows before it scrolls.
 * 0 hides the panel completely. Set by an admin from Settings. */
export const QUICK_ACCESS_MAX_ROWS = 8;
const storedQuickAccessRows = Number(localStorage.getItem("kashi-quick-access-rows"));
export const quickAccessRows = ref(
  localStorage.getItem("kashi-quick-access-rows") === null ||
    !Number.isInteger(storedQuickAccessRows)
    ? 3
    : Math.min(Math.max(storedQuickAccessRows, 0), QUICK_ACCESS_MAX_ROWS),
);

export function setQuickAccessRows(rows: number) {
  quickAccessRows.value = Math.min(Math.max(Math.round(rows), 0), QUICK_ACCESS_MAX_ROWS);
  localStorage.setItem("kashi-quick-access-rows", String(quickAccessRows.value));
}

/** Which lines the quick-access cards show. Set by an admin from Settings. */
export const quickAccessShowCode = ref(localStorage.getItem("kashi-quick-access-show-code") !== "false");
export const quickAccessShowName = ref(localStorage.getItem("kashi-quick-access-show-name") !== "false");

export function setQuickAccessShowCode(show: boolean) {
  quickAccessShowCode.value = show;
  localStorage.setItem("kashi-quick-access-show-code", String(show));
}

export function setQuickAccessShowName(show: boolean) {
  quickAccessShowName.value = show;
  localStorage.setItem("kashi-quick-access-show-name", String(show));
}

export const quickAccessShowRemove = ref(localStorage.getItem("kashi-quick-access-show-remove") !== "false");

export function setQuickAccessShowRemove(show: boolean) {
  quickAccessShowRemove.value = show;
  localStorage.setItem("kashi-quick-access-show-remove", String(show));
}

/** Swatches offered for quick-access card backgrounds in Settings. */
export const QUICK_ACCESS_COLORS = [
  "#ef9a9a", "#f48fb1", "#ce93d8", "#9fa8da", "#90caf9", "#80deea",
  "#a5d6a7", "#e6ee9c", "#fff59d", "#ffcc80", "#bcaaa4", "#b0bec5",
  "#c62828", "#ad1457", "#6a1b9a", "#283593", "#1565c0", "#00838f",
  "#2e7d32", "#9e9d24", "#f9a825", "#ef6c00", "#4e342e", "#37474f",
];

/** Text colour overrides offered in Settings: black, white, then the palette. */
export const QUICK_ACCESS_TEXT_COLORS = ["#000000", "#ffffff", ...QUICK_ACCESS_COLORS];

/** Black or white, whichever reads better on the given #rrggbb background. */
export function readableTextColor(hex: string) {
  const n = parseInt(hex.slice(1), 16);
  const [r, g, b] = [(n >> 16) & 255, (n >> 8) & 255, n & 255];
  return (r * 299 + g * 587 + b * 114) / 1000 > 150 ? "#000000" : "#ffffff";
}

/** Inline style for a quick-access card: the admin's background, and their
 * text colour or else black/white picked automatically for that background. */
export function quickAccessCardStyle(item: { quickAccessColor?: string; quickAccessTextColor?: string }) {
  const style: Record<string, string> = {};
  if (item.quickAccessColor) style.backgroundColor = item.quickAccessColor;
  const text = item.quickAccessTextColor || (item.quickAccessColor && readableTextColor(item.quickAccessColor));
  if (text) style.color = text;
  return style;
}

/** Text size of every quick-access card, as a percentage of normal. */
export const QUICK_ACCESS_FONT_MIN = 70;
export const QUICK_ACCESS_FONT_MAX = 200;
const storedQuickAccessFont = Number(localStorage.getItem("kashi-quick-access-font-scale"));
export const quickAccessFontScale = ref(
  Number.isFinite(storedQuickAccessFont) && storedQuickAccessFont >= QUICK_ACCESS_FONT_MIN
    ? Math.min(storedQuickAccessFont, QUICK_ACCESS_FONT_MAX)
    : 100,
);

export function setQuickAccessFontScale(percent: number) {
  quickAccessFontScale.value = Math.min(Math.max(Math.round(percent), QUICK_ACCESS_FONT_MIN), QUICK_ACCESS_FONT_MAX);
  localStorage.setItem("kashi-quick-access-font-scale", String(quickAccessFontScale.value));
}
