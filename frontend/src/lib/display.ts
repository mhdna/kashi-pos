import { ref } from "vue";
import { GetDisplayPrefs, ReportDisplayPrefs } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

export const isCompactBuild = import.meta.env.VITE_KASHI_COMPACT === "1";

const storedAnimations = localStorage.getItem("kashi-animations-enabled");
export const animationsEnabled = ref(
  storedAnimations === null ? !isCompactBuild : storedAnimations === "true",
);

export function setAnimationsEnabled(enabled: boolean) {
  animationsEnabled.value = enabled;
  localStorage.setItem("kashi-animations-enabled", String(enabled));
  reportDisplayPrefs();
}

/** Whether the sale screen's product search opens in place, its results
 * where the calculator is and its keyboard along the bottom, rather than as
 * a dialog over everything. On unless turned off in Settings. */
export const productSearchInPlace = ref(localStorage.getItem("kashi-product-search-in-place") !== "false");

export function setProductSearchInPlace(inPlace: boolean) {
  productSearchInPlace.value = inPlace;
  localStorage.setItem("kashi-product-search-in-place", String(inPlace));
  reportDisplayPrefs();
}

/** Currency code shown first on the calculator's total. Cycled by the
 * flip icon on the calculator itself, or set as a default from Settings. */
const storedTotalCurrency = localStorage.getItem("kashi-calculator-total-currency");
export const calculatorTotalCurrency = ref(storedTotalCurrency || "USD");

export function setCalculatorTotalCurrency(code: string) {
  calculatorTotalCurrency.value = code;
  localStorage.setItem("kashi-calculator-total-currency", code);
  reportDisplayPrefs();
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
  reportDisplayPrefs();
}

/** Which lines the quick-access cards show. Set by an admin from Settings. */
export const quickAccessShowCode = ref(localStorage.getItem("kashi-quick-access-show-code") !== "false");
export const quickAccessShowName = ref(localStorage.getItem("kashi-quick-access-show-name") !== "false");

export function setQuickAccessShowCode(show: boolean) {
  quickAccessShowCode.value = show;
  localStorage.setItem("kashi-quick-access-show-code", String(show));
  reportDisplayPrefs();
}

export function setQuickAccessShowName(show: boolean) {
  quickAccessShowName.value = show;
  localStorage.setItem("kashi-quick-access-show-name", String(show));
  reportDisplayPrefs();
}

export const quickAccessShowRemove = ref(localStorage.getItem("kashi-quick-access-show-remove") !== "false");

export function setQuickAccessShowRemove(show: boolean) {
  quickAccessShowRemove.value = show;
  localStorage.setItem("kashi-quick-access-show-remove", String(show));
  reportDisplayPrefs();
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
  reportDisplayPrefs();
}

/* ---- kashi-managed display prefs ------------------------------------------
 * The prefs above live in localStorage so they apply instantly, but they are
 * mirrored to the till's database (display_prefs.go) so kashi can see them
 * and push new values. A push bumps a revision; this screen adopts the stored
 * prefs whenever that revision is newer than the last one it applied. */

const APPLIED_REV_KEY = "kashi-display-prefs-rev";
let applying = false;

function currentDisplayPrefs() {
  return {
    quick_access_rows: quickAccessRows.value,
    quick_access_font_scale: quickAccessFontScale.value,
    quick_access_show_code: quickAccessShowCode.value,
    quick_access_show_name: quickAccessShowName.value,
    quick_access_show_remove: quickAccessShowRemove.value,
    animations_enabled: animationsEnabled.value,
    product_search_in_place: productSearchInPlace.value,
    calculator_total_currency: calculatorTotalCurrency.value,
  };
}

function reportDisplayPrefs() {
  if (applying) return;
  ReportDisplayPrefs(JSON.stringify(currentDisplayPrefs())).catch(() => {});
}

async function adoptPushedDisplayPrefs(applyTheme: (theme: string) => void) {
  const { prefs, rev } = await GetDisplayPrefs();
  if (rev <= Number(localStorage.getItem(APPLIED_REV_KEY) ?? 0)) return;
  const p = prefs ?? {};
  applying = true;
  try {
    if (typeof p.quick_access_rows === "number") setQuickAccessRows(p.quick_access_rows);
    if (typeof p.quick_access_font_scale === "number") setQuickAccessFontScale(p.quick_access_font_scale);
    if (typeof p.quick_access_show_code === "boolean") setQuickAccessShowCode(p.quick_access_show_code);
    if (typeof p.quick_access_show_name === "boolean") setQuickAccessShowName(p.quick_access_show_name);
    if (typeof p.quick_access_show_remove === "boolean") setQuickAccessShowRemove(p.quick_access_show_remove);
    if (typeof p.animations_enabled === "boolean") setAnimationsEnabled(p.animations_enabled);
    if (typeof p.product_search_in_place === "boolean") setProductSearchInPlace(p.product_search_in_place);
    if (typeof p.calculator_total_currency === "string" && p.calculator_total_currency) {
      setCalculatorTotalCurrency(p.calculator_total_currency);
    }
    if (p.theme === "light" || p.theme === "dark") applyTheme(p.theme);
  } finally {
    applying = false;
  }
  localStorage.setItem(APPLIED_REV_KEY, String(rev));
}

/** Called once the database is ready: adopt anything kashi pushed while the
 * app was closed, listen for live pushes, and report this screen's values. */
export async function startDisplayPrefsSync(applyTheme: (theme: string) => void) {
  const adopt = () => adoptPushedDisplayPrefs(applyTheme).catch(() => {});
  EventsOn("display_prefs_changed", adopt);
  await adopt();
  reportDisplayPrefs();
}
