// A product's colour is free text typed in kashi ("Grey", "Off-White",
// "Red/Tra", "pur/cuv", "#N/A"). This turns it into what a swatch paints:
// one CSS colour per part of a multi-colour name, in order.
//
// kashi's colours page has hex values, but the till doesn't receive them and
// the products' colour text doesn't always match those names, so the till
// reads the name itself. Parts it doesn't know are skipped; a name with no
// known part gets no colour (the swatch draws an empty, dashed circle).

const NAMED: Record<string, string> = {
  black: "#111111",
  white: "#ffffff",
  "off white": "#f4f1e8",
  offwhite: "#f4f1e8",
  ivory: "#fffbeb",
  cream: "#f5ecd2",
  ecru: "#efe6cf",
  grey: "#8a8a8a",
  gray: "#8a8a8a",
  silver: "#c0c0c0",
  charcoal: "#3c3f44",
  anthracite: "#383e42",
  beige: "#d9c7a7",
  greige: "#bfb5a5",
  nude: "#e3bc9a",
  sand: "#d6c29a",
  camel: "#c19a6b",
  khaki: "#b5a76a",
  taupe: "#8b7d6b",
  tan: "#d2b48c",
  brown: "#6f4e37",
  chocolate: "#4e2e1e",
  coffee: "#6f4e37",
  mocha: "#7b5a45",
  navy: "#1f2a52",
  blue: "#2463eb",
  "light blue": "#9cc3ec",
  "sky blue": "#87ceeb",
  sky: "#87ceeb",
  "royal blue": "#2a52be",
  royal: "#2a52be",
  "dark blue": "#1e3a8a",
  denim: "#3b5b8c",
  indigo: "#3f3d9b",
  petrol: "#1f5f6b",
  teal: "#0f8b8d",
  turquoise: "#30c5c0",
  aqua: "#4fd1d9",
  mint: "#a8e6cf",
  green: "#2e9e4f",
  "dark green": "#1f5130",
  olive: "#708238",
  lime: "#9acd32",
  yellow: "#f5d000",
  mustard: "#d8a31a",
  gold: "#d4af37",
  orange: "#f28c28",
  coral: "#ff7f6a",
  peach: "#ffcba4",
  salmon: "#fa8072",
  red: "#d62828",
  bordeaux: "#6d1a2b",
  burgundy: "#6d1a2b",
  maroon: "#7a1f2b",
  wine: "#722f37",
  pink: "#f4a6c1",
  "light pink": "#f9cfdc",
  "baby pink": "#f9cfdc",
  rose: "#e8879c",
  fuchsia: "#d61f84",
  magenta: "#c2187a",
  purple: "#7b3fa0",
  violet: "#8a4fd8",
  lilac: "#c8a2c8",
  lavender: "#b9a5e3",
  plum: "#7d3c63",
  transparent: "transparent",
  clear: "transparent",
};

// Short forms seen in imported catalogues ("Pur/Tra", "BLU/SHW", "Yel/Cuv").
const SHORT: Record<string, string> = {
  blk: "black",
  bk: "black",
  wht: "white",
  wh: "white",
  gry: "grey",
  gy: "grey",
  nvy: "navy",
  nav: "navy",
  blu: "blue",
  bl: "blue",
  pur: "purple",
  ppl: "purple",
  yel: "yellow",
  ylw: "yellow",
  ora: "orange",
  org: "orange",
  grn: "green",
  brn: "brown",
  bge: "beige",
  pnk: "pink",
  rd: "red",
  bord: "bordeaux",
  tra: "transparent",
  trans: "transparent",
  transp: "transparent",
};

// Rainbow, for "Colored" / "Multi": several colours, none in particular.
const MULTI = ["#d62828", "#f28c28", "#f5d000", "#2e9e4f", "#2463eb", "#7b3fa0"];
const MULTI_WORDS = new Set(["colored", "coloured", "multi", "multicolor", "multicolour", "multi color", "multi colour", "mix", "mixed", "assorted"]);

const HEX = /^#(?:[0-9a-f]{3}|[0-9a-f]{6})$/i;

function lookup(word: string): string | null {
  if (NAMED[word]) return NAMED[word];
  if (SHORT[word]) return NAMED[SHORT[word]!]!;
  return null;
}

function partColour(raw: string): string[] | null {
  const part = raw.trim();
  if (!part) return null;
  if (HEX.test(part)) return [part.toLowerCase()];
  const words = part
    .toLowerCase()
    .replace(/[-_]+/g, " ")
    .replace(/\s+/g, " ")
    .trim();
  if (MULTI_WORDS.has(words)) return MULTI;
  const whole = lookup(words);
  if (whole) return [whole];
  // "Light Gray", "Dark Navy": the colour word, lightened or darkened.
  const tokens = words.split(" ");
  for (let i = tokens.length - 1; i >= 0; i--) {
    const base = lookup(tokens[i]!);
    if (!base) continue;
    const before = tokens.slice(0, i);
    if (base !== "transparent" && before.includes("light")) return [mix(base, "#ffffff", 0.45)];
    if (base !== "transparent" && before.includes("dark")) return [mix(base, "#000000", 0.4)];
    return [base];
  }
  return null;
}

/** The CSS colours a swatch for this colour name paints, in order; empty when none is known. */
export function colourStops(name: string | null | undefined): string[] {
  const text = String(name ?? "").trim();
  if (!text || /^#?n\/?a$/i.test(text)) return [];
  if (HEX.test(text)) return [text.toLowerCase()];
  const stops: string[] = [];
  for (const part of text.split(/[/,&+|]/)) {
    const colours = partColour(part);
    if (!colours) continue;
    for (const c of colours) if (stops[stops.length - 1] !== c) stops.push(c);
  }
  return stops;
}

/**
 * The CSS background for a round swatch: one colour fills it, several share
 * it in slices. Transparent shows as a checkerboard.
 */
export function swatchBackground(stops: string[]): string {
  if (!stops.length) return "transparent";
  const paint = stops.map((s) => (s === "transparent" ? "#ffffff" : s));
  const checker = stops.includes("transparent")
    ? ", repeating-conic-gradient(#d0d0d0 0% 25%, #ffffff 0% 50%) 0 0 / 8px 8px"
    : "";
  if (paint.length === 1) {
    return stops[0] === "transparent" ? checker.slice(2) : paint[0]!;
  }
  const slice = 100 / paint.length;
  const parts = paint.map((c, i) => {
    const colour = stops[i] === "transparent" ? "transparent" : c;
    return `${colour} ${(i * slice).toFixed(2)}% ${((i + 1) * slice).toFixed(2)}%`;
  });
  return `conic-gradient(${parts.join(", ")})${checker}`;
}

function mix(hex: string, with_: string, amount: number): string {
  const a = toRgb(hex);
  const b = toRgb(with_);
  const c = a.map((v, i) => Math.round(v + (b[i]! - v) * amount));
  return `#${c.map((v) => v.toString(16).padStart(2, "0")).join("")}`;
}

function toRgb(hex: string): [number, number, number] {
  let h = hex.slice(1);
  if (h.length === 3) h = h.replace(/./g, (x) => x + x);
  return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16)];
}
