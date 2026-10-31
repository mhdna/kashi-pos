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

export const roundedValue: Record<string, string> = {
  "0": "0",
  sm: "2px",
  lg: "8px",
  xl: "24px",
  pill: "9999px",
  circle: "50%",
};

export function roundedStyle(rounded?: string | boolean): string | undefined {
  if (rounded === undefined || rounded === false) return undefined;
  if (rounded === true || rounded === "") return "4px";
  return roundedValue[rounded] ?? rounded;
}

export const namedSizes = ["x-small", "small", "large", "x-large"];
