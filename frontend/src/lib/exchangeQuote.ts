// What the exchange screen shows, from the backend's exchange quote
// (invoice_quote.go ExchangeQuote, exchange_quote_lines.go). Nothing here is
// priced: every amount is the backend's, only laid out. The one rule is that
// what is on screen adds up to the total on screen, and when it can't (a
// stale or broken quote) the screen says so instead of showing numbers that
// don't.

// One cart line as the quote priced it, in the order the cart sent it.
export interface ExchangeQuoteLine {
  index: number;
  code: string;
  barcode: string;
  // Below zero for a returned line.
  qty: number;
  // What the line adds to the exchange: negative for a returned line's credit.
  lineTotalCents: number;
  // The shelf price of its units, signed like the line.
  shelfCents: number;
  // A returned line's credit is paidCents + swapCreditCents - chargedBackCents.
  paidCents?: number;
  swapCreditCents?: number;
  chargedBackCents?: number;
  // What a free unit could take off the new items (its shelf price), used
  // or not; swapCreditCents is the part used.
  swapAllowanceCents?: number;
  deal?: string;
  free?: boolean;
}

export interface ExchangeQuoteAdjustment {
  deal: string;
  chargedBackCents?: number;
  forfeitedCents?: number;
}

export interface ExchangeQuoteResult {
  returnedTotalCents: number;
  replacementTotalCents: number;
  netCents: number;
  taxCents: number;
  unroundedCents: number;
  roundingCents: number;
  grandTotalCents: number;
  needsMoreReplacementValue: boolean;
  shortfallCents: number;
  lines: ExchangeQuoteLine[];
  adjustments: ExchangeQuoteAdjustment[];
}

// A cart line, as far as matching it to its quote line goes.
export interface ExchangeCartLine {
  code: string;
  barcode?: string;
  qty: number;
}

const cents = (value: unknown): number => {
  const n = Number(value ?? 0);
  return Number.isFinite(n) ? Math.round(n) : NaN;
};

// The quote as Wails hands it over (numbers may come as anything JSON
// gives), with every amount a whole number of cents.
export function normalizeExchangeQuote(raw: any): ExchangeQuoteResult {
  return {
    returnedTotalCents: cents(raw?.returnedTotalCents),
    replacementTotalCents: cents(raw?.replacementTotalCents),
    netCents: cents(raw?.netCents),
    taxCents: cents(raw?.taxCents),
    unroundedCents: cents(raw?.unroundedCents),
    roundingCents: cents(raw?.roundingCents),
    grandTotalCents: cents(raw?.grandTotalCents),
    needsMoreReplacementValue: Boolean(raw?.needsMoreReplacementValue),
    shortfallCents: cents(raw?.shortfallCents),
    lines: (raw?.lines ?? []).map((l: any) => ({
      index: Number(l.index),
      code: String(l.code ?? ""),
      barcode: String(l.barcode ?? ""),
      qty: Number(l.qty),
      lineTotalCents: cents(l.lineTotalCents),
      shelfCents: cents(l.shelfCents),
      paidCents: cents(l.paidCents),
      swapCreditCents: cents(l.swapCreditCents),
      chargedBackCents: cents(l.chargedBackCents),
      swapAllowanceCents: cents(l.swapAllowanceCents),
      deal: l.deal ? String(l.deal) : undefined,
      free: Boolean(l.free),
    })),
    adjustments: (raw?.adjustments ?? []).map((a: any) => ({
      deal: String(a.deal ?? ""),
      chargedBackCents: cents(a.chargedBackCents),
      forfeitedCents: cents(a.forfeitedCents),
    })),
  };
}

// Why a quote can't be shown as it is, or "" when everything adds up:
// the lines to the net, net and tax to the unrounded total, that and the
// rounding to the total. Mirrors the backend's own checks, so a quote that
// slipped past them (or an app half-updated) still never shows a wrong sum.
export function exchangeQuoteProblem(q: ExchangeQuoteResult, cart: ExchangeCartLine[]): string {
  const amounts = [
    q.returnedTotalCents,
    q.replacementTotalCents,
    q.netCents,
    q.taxCents,
    q.unroundedCents,
    q.roundingCents,
    q.grandTotalCents,
    q.shortfallCents,
  ];
  if (amounts.some((n) => !Number.isInteger(n))) return "the exchange came back without amounts";
  if (q.lines.length !== cart.length) return "the exchange was priced for a different cart";
  let sum = 0;
  let returned = 0;
  let replacements = 0;
  for (const [i, l] of q.lines.entries()) {
    if (!matchesCartLine(l, cart[i], i)) return "the exchange was priced for a different cart";
    if (!Number.isInteger(l.lineTotalCents)) return "a line came back without an amount";
    sum += l.lineTotalCents;
    if (l.qty < 0) {
      returned -= l.lineTotalCents;
      const parts = (l.paidCents ?? 0) + (l.swapCreditCents ?? 0) - (l.chargedBackCents ?? 0);
      if (parts !== -l.lineTotalCents) return `${l.code}'s credit doesn't add up`;
      if ((l.swapCreditCents ?? 0) > (l.swapAllowanceCents ?? 0)) {
        return `${l.code}'s free item credit is more than it allows`;
      }
    } else {
      replacements += l.lineTotalCents;
    }
  }
  if (returned !== q.returnedTotalCents || replacements !== q.replacementTotalCents) {
    return "the lines don't add up to the exchange";
  }
  if (sum !== q.netCents || q.replacementTotalCents - q.returnedTotalCents !== q.netCents) {
    return "the lines don't add up to the exchange";
  }
  if (q.netCents + q.taxCents !== q.unroundedCents) return "the tax doesn't add up";
  if (Math.max(0, q.unroundedCents + q.roundingCents) !== q.grandTotalCents)
    return "the total doesn't add up";
  const lineBack = q.lines.reduce((s, l) => s + (l.chargedBackCents ?? 0), 0);
  const dealBack = q.adjustments.reduce((s, a) => s + (a.chargedBackCents ?? 0), 0);
  if (lineBack !== dealBack) return "the deal charge back doesn't add up";
  // "Add $X more" is what the lines on screen are short of, to the cent;
  // and asking for nothing more, the lines never come to a refund.
  if (q.needsMoreReplacementValue !== q.shortfallCents > 0) return "the shortfall doesn't add up";
  if (q.needsMoreReplacementValue && q.shortfallCents !== -q.netCents) {
    return "the shortfall doesn't match the lines";
  }
  if (!q.needsMoreReplacementValue && q.netCents < 0) return "the lines come to a refund";
  return "";
}

function matchesCartLine(
  l: ExchangeQuoteLine,
  c: ExchangeCartLine | undefined,
  index: number,
): boolean {
  if (!c || l.index !== index || l.qty !== c.qty) return false;
  if (
    l.code.trim().toLowerCase() !==
    String(c.code ?? "")
      .trim()
      .toLowerCase()
  )
    return false;
  return (l.barcode || "") === String(c.barcode ?? "").trim();
}

// The backend's answer for the cart JSON it was asked about: the quote,
// normalized, when it adds up for that cart; otherwise the reason, for the
// calculator to show in place of a total (which also blocks checkout).
export function checkExchangeQuote(
  raw: unknown,
  itemsJson: string,
): { quote: ExchangeQuoteResult; error: "" } | { quote: null; error: string } {
  let cart: ExchangeCartLine[];
  try {
    cart = JSON.parse(itemsJson);
  } catch {
    return { quote: null, error: "Can't show this exchange: the cart couldn't be read" };
  }
  const quote = normalizeExchangeQuote(raw);
  const problem = exchangeQuoteProblem(quote, Array.isArray(cart) ? cart : []);
  if (problem) return { quote: null, error: `Can't show this exchange: ${problem}` };
  return { quote, error: "" };
}

// The quote's line for the cart line at index, or undefined when the quote
// isn't for this cart (it is being re-priced): the screen then shows the
// line as pending rather than a number that may be wrong.
export function exchangeLineAt(
  q: ExchangeQuoteResult | null | undefined,
  cart: ExchangeCartLine[],
  index: number,
): ExchangeQuoteLine | undefined {
  if (!q || exchangeQuoteProblem(q, cart)) return undefined;
  return q.lines[index];
}

// "$5.67", "−$5.67", "+$0.01": two decimals always, so a column of them lines up.
export function formatCents(value: number, sign: "auto" | "always" = "auto"): string {
  const abs = Math.abs(value);
  const text = `$${Math.trunc(abs / 100).toLocaleString("en-US")}.${String(abs % 100).padStart(2, "0")}`;
  if (value < 0) return `−${text}`;
  return sign === "always" ? `+${text}` : text;
}

// Under a returned line, how its credit came about, when it isn't simply
// its shelf price: "Paid $12.00 · Buy 2 get 1 charged back $6.33". A free
// unit says what it can still take off new items: it is worth nothing
// until something is bought, then up to its shelf price.
// [] when the credit is the shelf price.
// How a returned line's credit is coloured, so a cashier sees at a glance
// why it isn't the shelf price: "charged" (orange) when a deal saving came
// off it, "free" (green) for a free item's credit toward the new items, ""
// when it is simply what was paid.
export type ReturnedLineTone = "charged" | "free" | "";

export function returnedLineTone(l: ExchangeQuoteLine): ReturnedLineTone {
  if (l.qty >= 0) return "";
  if ((l.chargedBackCents ?? 0) > 0) return "charged";
  if (l.free) return "free";
  return "";
}

export function returnedLineNotes(l: ExchangeQuoteLine): string[] {
  if (l.qty >= 0) return [];
  const paid = l.paidCents ?? 0;
  const swap = l.swapCreditCents ?? 0;
  const allowance = l.swapAllowanceCents ?? 0;
  const back = l.chargedBackCents ?? 0;
  const credit = -l.lineTotalCents;
  if (credit === -l.shelfCents && swap === 0 && back === 0 && !l.free) return [];
  const notes: string[] = [];
  if (l.free) {
    if (allowance <= 0) notes.push("Free item");
    else if (swap === 0) notes.push(`Free item · up to ${formatCents(allowance)} off new items`);
    else if (swap < allowance) {
      notes.push(
        `Free item · ${formatCents(swap)} of its ${formatCents(allowance)} off the new items`,
      );
    } else notes.push(`Free item · ${formatCents(swap)} off the new items`);
  } else {
    notes.push(`Paid ${formatCents(paid)}`);
    if (swap > 0) notes.push(`${formatCents(swap)} off the new items`);
  }
  if (back > 0) notes.push(`${l.deal || "Deal"} charged back ${formatCents(back)}`);
  return [notes.join(" · ")];
}

export interface ExchangeSummaryRow {
  key: string;
  label: string;
  cents: number;
  // Shown smaller, under the row it explains; not part of the sum.
  note?: boolean;
  total?: boolean;
}

// The breakdown under the cart: new items, the returned credit, what the
// deals did (notes), tax, rounding, and the total. The rows that aren't
// notes add up to the total row, always; when the replacements don't cover
// the returned items yet, the total is 0 and the shortfall says why.
export function exchangeSummaryRows(q: ExchangeQuoteResult): ExchangeSummaryRow[] {
  const rows: ExchangeSummaryRow[] = [];
  rows.push({ key: "new", label: "New items", cents: q.replacementTotalCents });
  rows.push({ key: "returned", label: "Returned credit", cents: -q.returnedTotalCents });
  for (const a of q.adjustments) {
    if ((a.chargedBackCents ?? 0) > 0) {
      rows.push({
        key: `back-${a.deal}`,
        label: `${a.deal} no longer applies: ${formatCents(a.chargedBackCents ?? 0)} charged back`,
        cents: a.chargedBackCents ?? 0,
        note: true,
      });
    }
    // A free item's credit is only lost once the new items are in: while
    // more are needed it is still there to use (the line says "up to").
    if ((a.forfeitedCents ?? 0) > 0 && !q.needsMoreReplacementValue) {
      rows.push({
        key: `forfeit-${a.deal}`,
        label: `Unused free item credit (${a.deal}): ${formatCents(a.forfeitedCents ?? 0)}`,
        cents: a.forfeitedCents ?? 0,
        note: true,
      });
    }
  }
  if (q.taxCents !== 0) rows.push({ key: "tax", label: "Tax", cents: q.taxCents });
  if (q.roundingCents !== 0)
    rows.push({ key: "rounding", label: "Cash rounding", cents: q.roundingCents });
  if (q.needsMoreReplacementValue) {
    rows.push({
      key: "shortfall",
      label: `Add ${formatCents(q.shortfallCents)} more in new items`,
      cents: q.shortfallCents,
      note: true,
    });
  }
  rows.push({ key: "total", label: "To pay", cents: q.grandTotalCents, total: true });
  return rows;
}

// The rows' sum, for the tests and as a last check before showing them.
export function summaryAddsUp(rows: ExchangeSummaryRow[]): boolean {
  const total = rows.find((r) => r.total);
  if (!total) return false;
  const sum = rows.filter((r) => !r.note && !r.total).reduce((s, r) => s + r.cents, 0);
  return Math.max(0, sum) === total.cents;
}
