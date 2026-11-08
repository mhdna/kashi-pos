// kashi's live cart (live_cart.go), as the cart sees it: which of kashi's
// overrides a line takes, and how the line says so.

// One line of kashi's overrides, as GetLiveCartOverrides returns it. A
// product's goes by its barcode; an unknown item's by its line ref (the
// cart's unknownRef), since every unknown item shares one barcode.
export interface LiveCartLineOverride {
  barcode: string;
  line_ref?: string;
  price_cents?: number | null;
  discount?: number | null;
  no_deal?: boolean;
  // An unknown item's deal choice: b2g1_30, b2g1_20 or none.
  unknown_deal?: string;
}

export interface LiveCartLine {
  barcode?: string;
  qty: number;
  locked?: boolean;
  unknownRef?: string;
}

// The override kashi set on this line, if any. An unknown item without a ref
// (an old draft or hold) can't be named, so it takes none.
export function liveOverrideFor(
  line: LiveCartLine,
  overrides: LiveCartLineOverride[],
  unknown: boolean,
): LiveCartLineOverride | undefined {
  if (!(line.qty > 0) || line.locked || !line.barcode) return undefined;
  if (unknown) {
    if (!line.unknownRef) return undefined;
    return overrides.find((o) => o.barcode === line.barcode && o.line_ref === line.unknownRef);
  }
  return overrides.find((o) => o.barcode === line.barcode && !o.line_ref);
}

const unknownDealNames: Record<string, string> = {
  b2g1_30: "deal, else 30%",
  b2g1_20: "deal, else 20%",
  none: "no deal",
};

// "Set in kashi: price $12.00, 10% off, no deal" for the line's lock chip.
export function liveOverrideTitle(o: {
  priceCents?: number | null;
  discount?: number | null;
  noDeal?: boolean;
  unknownDeal?: string;
}): string {
  const parts = [];
  if (o.priceCents != null) parts.push(`price $${(o.priceCents / 100).toFixed(2)}`);
  if (o.discount != null) parts.push(`${o.discount}% off`);
  if (o.noDeal) parts.push("no deal");
  else if (o.unknownDeal) parts.push(unknownDealNames[o.unknownDeal] ?? o.unknownDeal);
  return `Set in kashi: ${parts.join(", ")}`;
}
