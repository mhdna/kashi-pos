import { palette } from "./palette";

const themeColors = new Set([
  "primary",
  "secondary",
  "error",
  "info",
  "success",
  "warning",
  "surface",
  "surface-light",
  "surface-variant",
  "background",
]);

// Turns a `color` prop into a CSS colour. Theme names follow the live theme,
// Material names ("blue-grey-darken-1") come from the palette, and anything
// else (#hex, rgb(), var()) passes straight through.
export function resolveColor(color: string | undefined | null): string | undefined {
  if (!color) return undefined;
  if (themeColors.has(color)) return `rgb(var(--k-theme-${color}))`;
  return palette[color] ?? color;
}

// The text colour to use on top of a solid fill of this colour.
export function onColor(color: string | undefined | null): string {
  if (!color) return "rgb(var(--k-theme-on-surface))";
  if (themeColors.has(color)) {
    return color === "surface" || color === "background" || color === "surface-light"
      ? "rgb(var(--k-theme-on-surface))"
      : color === "surface-variant"
        ? "rgb(var(--k-theme-on-surface-variant))"
        : `rgb(var(--k-theme-on-${color}))`;
  }
  const hex = palette[color] ?? color;
  const m = /^#([0-9a-f]{6})$/i.exec(hex);
  if (!m) return "#fff";
  const n = parseInt(m[1]!, 16);
  const luminance = (0.299 * ((n >> 16) & 255) + 0.587 * ((n >> 8) & 255) + 0.114 * (n & 255)) / 255;
  return luminance > 0.62 ? "#000" : "#fff";
}
