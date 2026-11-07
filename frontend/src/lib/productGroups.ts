// The product search shows the variants sharing a code (one row per colour
// and size in the products table, see migration 000027) as one result, and
// the cashier picks the colour and size from it. This is that grouping and
// the picker's rules, kept out of the components so they can be tested.

export interface VariantLike {
  id: number;
  code: string;
  color?: string;
  size?: string;
  price?: number;
}

export interface ProductGroup<T extends VariantLike = VariantLike> {
  /** The code, or "id:<id>" for a product with no code (its own group). */
  key: string;
  /** The first variant found: its code, name and price stand for the group. */
  first: T;
  variants: T[];
  colors: string[];
  sizes: string[];
  minPrice: number;
  maxPrice: number;
}

export function groupKey(p: VariantLike): string {
  return p.code ? p.code : `id:${p.id}`;
}

/** Variants grouped by code, groups in the order their first variant came. */
export function groupProducts<T extends VariantLike>(products: readonly T[]): ProductGroup<T>[] {
  const byKey = new Map<string, T[]>();
  for (const p of products) {
    const key = groupKey(p);
    const list = byKey.get(key);
    if (list) list.push(p);
    else byKey.set(key, [p]);
  }
  return [...byKey].map(([key, variants]) => summarise(key, variants));
}

export function summarise<T extends VariantLike>(key: string, variants: T[]): ProductGroup<T> {
  const prices = variants.map((v) => Number(v.price || 0));
  return {
    key,
    first: variants[0]!,
    variants,
    colors: distinct(variants.map((v) => v.color)),
    sizes: sortSizes(distinct(variants.map((v) => v.size))),
    minPrice: Math.min(...prices),
    maxPrice: Math.max(...prices),
  };
}

function distinct(values: (string | undefined)[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of values) {
    const v = (raw ?? "").trim();
    if (!v || seen.has(v.toLowerCase())) continue;
    seen.add(v.toLowerCase());
    out.push(v);
  }
  return out;
}

const LETTER_SIZES = ["xxxs", "xxs", "xs", "s", "m", "l", "xl", "xxl", "2xl", "xxxl", "3xl", "4xl", "5xl"];
const LETTER_RANK: Record<string, number> = { "2xl": 7, "3xl": 9 };
LETTER_SIZES.forEach((s, i) => (LETTER_RANK[s] ??= i));

function sizeRank(size: string): [number, number] {
  const s = size.trim().toLowerCase();
  if (s in LETTER_RANK) return [0, LETTER_RANK[s]!];
  const n = /^(\d+(?:[.,]\d+)?)/.exec(s);
  if (n) return [1, parseFloat(n[1]!.replace(",", "."))];
  return [2, 0];
}

/** Letter sizes in wearing order, then numbers (and ranges) by their first number, then the rest as they came. */
export function sortSizes(sizes: string[]): string[] {
  return sizes
    .map((s, i) => ({ s, i, r: sizeRank(s) }))
    .sort((a, b) => a.r[0] - b.r[0] || a.r[1] - b.r[1] || a.i - b.i)
    .map((x) => x.s);
}

const same = (a: string | undefined, b: string | null) => (a ?? "").trim().toLowerCase() === (b ?? "").trim().toLowerCase();

/** The variants a colour and size choice (either may still be open: null) leaves. */
export function matching<T extends VariantLike>(variants: readonly T[], color: string | null, size: string | null): T[] {
  return variants.filter((v) => (color === null || same(v.color, color)) && (size === null || same(v.size, size)));
}

/** Whether picking this colour, with the size chosen so far, leaves any variant. */
export function colorAvailable(variants: readonly VariantLike[], color: string, size: string | null): boolean {
  return matching(variants, color, size).length > 0;
}

export function sizeAvailable(variants: readonly VariantLike[], size: string, color: string | null): boolean {
  return matching(variants, color, size).length > 0;
}

/** "$29.00", or "$29.00 – $35.00" when the variants differ. */
export function priceLabel(min: number, max: number): string {
  const f = (n: number) => `$${n.toFixed(2)}`;
  return min === max ? f(min) : `${f(min)} – ${f(max)}`;
}
