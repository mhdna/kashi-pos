import { onColor, resolveColor } from "./colors";

// CSS custom properties a component sets from its `color` prop; the stylesheet
// reads --k-c (the colour) and --k-on (readable text on top of that colour).
export function colorVars(color?: string | null): Record<string, string> | undefined {
  if (!color) return undefined;
  return { "--k-c": resolveColor(color)!, "--k-on": onColor(color) };
}

// Numbers become px, other strings pass through.
export function toUnit(value?: string | number | null): string | undefined {
  if (value === undefined || value === null || value === "") return undefined;
  if (typeof value === "number") return `${value}px`;
  return /^-?\d+(\.\d+)?$/.test(value) ? `${value}px` : value;
}

export function roundedStyle(rounded?: string | boolean): string | undefined {
  if (rounded === undefined || rounded === false) return undefined;
  // Nothing in the app has rounded corners: only a true circle stays round.
  return rounded === "circle" ? "50%" : "0";
}

export const namedSizes = ["x-small", "small", "large", "x-large"];
