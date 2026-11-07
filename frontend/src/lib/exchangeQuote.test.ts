import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
  checkExchangeQuote,
  exchangeLineAt,
  exchangeQuoteProblem,
  exchangeSummaryRows,
  formatCents,
  normalizeExchangeQuote,
  returnedLineNotes,
  returnedLineTone,
  summaryAddsUp,
  type ExchangeCartLine,
  type ExchangeQuoteLine,
  type ExchangeQuoteResult,
} from "./exchangeQuote";

// The exchange from the shop floor: an unknown item at $30 for a returned
// Lady Winter Vizon ($12 on the shelf) that was a paid unit of Buy 2 get 1;
// with it gone the deal is charged back $6.33, so it credits $5.67 and the
// customer pays $24.33.
const cart: ExchangeCartLine[] = [
  { code: "UNKNOWN", barcode: "UNKNOWN", qty: 1 },
  { code: "CA24LWVZ12", barcode: "bc-vizon", qty: -1 },
];
function shopFloorQuote(): ExchangeQuoteResult {
  return normalizeExchangeQuote({
    returnedTotalCents: 567,
    replacementTotalCents: 3000,
    netCents: 2433,
    taxCents: 0,
    unroundedCents: 2433,
    roundingCents: 0,
    grandTotalCents: 2433,
    needsMoreReplacementValue: false,
    shortfallCents: 0,
    lines: [
      {
        index: 0,
        code: "UNKNOWN",
        barcode: "UNKNOWN",
        qty: 1,
        lineTotalCents: 3000,
        shelfCents: 3000,
      },
      {
        index: 1,
        code: "CA24LWVZ12",
        barcode: "bc-vizon",
        qty: -1,
        lineTotalCents: -567,
        shelfCents: -1200,
        paidCents: 1200,
        chargedBackCents: 633,
        deal: "Buy 2 get 1",
      },
    ],
    adjustments: [{ deal: "Buy 2 get 1", chargedBackCents: 633 }],
  });
}

describe("formatCents", () => {
  it.each([
    [0, "$0.00"],
    [5, "$0.05"],
    [567, "$5.67"],
    [-567, "−$5.67"],
    [100, "$1.00"],
    [123456789, "$1,234,567.89"],
    [-1, "−$0.01"],
  ])("%i is %s", (cents, text) => {
    expect(formatCents(cents)).toBe(text);
  });
  it("can always sign", () => {
    expect(formatCents(1, "always")).toBe("+$0.01");
    expect(formatCents(-1, "always")).toBe("−$0.01");
  });
  it("reads back to the same cents for every amount", () => {
    for (let c = -100_000; c <= 100_000; c += 37) {
      const text = formatCents(c);
      const back =
        Math.round(Number(text.replace(/[−$,]/g, "")) * 100) * (text.startsWith("−") ? -1 : 1);
      expect(back).toBe(c);
    }
  });
});

describe("the shop-floor exchange", () => {
  it("adds up", () => {
    expect(exchangeQuoteProblem(shopFloorQuote(), cart)).toBe("");
  });

  it("shows the returned line's credit, not its shelf price", () => {
    const line = exchangeLineAt(shopFloorQuote(), cart, 1)!;
    expect(line.lineTotalCents).toBe(-567);
    expect(returnedLineNotes(line)).toEqual(["Paid $12.00 · Buy 2 get 1 charged back $6.33"]);
  });

  it("shows a breakdown that ends in the total", () => {
    const rows = exchangeSummaryRows(shopFloorQuote());
    expect(rows.map((r) => [r.label, r.cents, Boolean(r.note)])).toEqual([
      ["New items", 3000, false],
      ["Returned credit", -567, false],
      ["Buy 2 get 1 no longer applies: $6.33 charged back", 633, true],
      ["To pay", 2433, false],
    ]);
    expect(summaryAddsUp(rows)).toBe(true);
  });

  it("the cart's line amounts add up to the total", () => {
    const q = shopFloorQuote();
    const sum = cart.reduce((s, _, i) => s + exchangeLineAt(q, cart, i)!.lineTotalCents, 0);
    expect(sum).toBe(q.grandTotalCents);
  });
});

describe("a quote that doesn't add up is never shown", () => {
  // Every amount the screen shows or sums, moved by one cent.
  const mutations: [string, (q: ExchangeQuoteResult) => void][] = [
    ["returned total", (q) => (q.returnedTotalCents += 1)],
    ["replacement total", (q) => (q.replacementTotalCents += 1)],
    ["net", (q) => (q.netCents += 1)],
    ["tax", (q) => (q.taxCents += 1)],
    ["unrounded", (q) => (q.unroundedCents += 1)],
    ["rounding", (q) => (q.roundingCents += 1)],
    ["grand total", (q) => (q.grandTotalCents += 1)],
    ["a replacement line", (q) => (q.lines[0]!.lineTotalCents += 1)],
    ["a returned line", (q) => (q.lines[1]!.lineTotalCents += 1)],
    ["what was paid", (q) => (q.lines[1]!.paidCents! += 1)],
    ["a swap credit", (q) => (q.lines[1]!.swapCreditCents! += 1)],
    ["a line's charge back", (q) => (q.lines[1]!.chargedBackCents! += 1)],
    ["the deal's charge back", (q) => (q.adjustments[0]!.chargedBackCents! += 1)],
    ["a line's index", (q) => (q.lines[1]!.index = 0)],
    ["a line's quantity", (q) => (q.lines[1]!.qty = -2)],
    ["a line's code", (q) => (q.lines[1]!.code = "OTHER")],
    ["a line's barcode", (q) => (q.lines[1]!.barcode = "other")],
    ["a missing line", (q) => q.lines.pop()],
    [
      "a shortfall the lines don't have",
      (q) => {
        q.needsMoreReplacementValue = true;
        q.shortfallCents = 1;
      },
    ],
    [
      "a shortfall off by a cent",
      (q) => {
        q.lines[0]!.lineTotalCents = 0;
        q.replacementTotalCents = 0;
        q.netCents = q.unroundedCents = -567;
        q.grandTotalCents = 0;
        q.needsMoreReplacementValue = true;
        q.shortfallCents = 568;
      },
    ],
    [
      "a refund with nothing more asked",
      (q) => {
        q.lines[0]!.lineTotalCents = 0;
        q.replacementTotalCents = 0;
        q.netCents = q.unroundedCents = -567;
        q.grandTotalCents = 0;
      },
    ],
    [
      "a free credit over its allowance",
      (q) => {
        q.lines[1]!.swapCreditCents = 1;
        q.lines[1]!.chargedBackCents = 634;
        q.adjustments[0]!.chargedBackCents = 634;
      },
    ],
    ["an amount that isn't one", (q) => (q.netCents = NaN)],
  ];
  it.each(mutations)("catches %s", (_, mutate) => {
    const q = shopFloorQuote();
    mutate(q);
    expect(exchangeQuoteProblem(q, cart)).not.toBe("");
    expect(exchangeLineAt(q, cart, 0)).toBeUndefined();
  });

  it("is case- and space-blind on codes, as the backend is", () => {
    const q = shopFloorQuote();
    expect(exchangeQuoteProblem(q, [{ ...cart[0]!, code: " unknown " }, cart[1]!])).toBe("");
  });

  it("shows nothing for a cart that changed since it was priced", () => {
    const q = shopFloorQuote();
    expect(exchangeLineAt(q, [cart[1]!, cart[0]!], 0)).toBeUndefined();
    expect(exchangeLineAt(q, [...cart, { code: "X", qty: 1 }], 0)).toBeUndefined();
    expect(exchangeLineAt(q, [{ ...cart[0]!, qty: 2 }, cart[1]!], 1)).toBeUndefined();
    expect(exchangeLineAt(null, cart, 0)).toBeUndefined();
  });
});

describe("checkExchangeQuote", () => {
  it("passes a quote for the cart it was asked about", () => {
    const checked = checkExchangeQuote(shopFloorQuote(), JSON.stringify(cart));
    expect(checked.error).toBe("");
    expect(checked.quote?.grandTotalCents).toBe(2433);
  });
  it("takes numbers sent as strings", () => {
    const raw = JSON.parse(JSON.stringify(shopFloorQuote()), (_, v) =>
      typeof v === "number" ? String(v) : v,
    );
    raw.lines.forEach((l: any) => (l.index = Number(l.index)));
    expect(checkExchangeQuote(raw, JSON.stringify(cart)).error).toBe("");
  });
  it("turns anything else into an error, never a total", () => {
    for (const raw of [
      null,
      {},
      { ...shopFloorQuote(), lines: [] },
      { ...shopFloorQuote(), grandTotalCents: 1 },
    ]) {
      const checked = checkExchangeQuote(raw, JSON.stringify(cart));
      expect(checked.quote).toBeNull();
      expect(checked.error).toMatch(/^Can't show this exchange: /);
    }
    expect(checkExchangeQuote(shopFloorQuote(), "not json").quote).toBeNull();
  });
});

describe("returnedLineNotes", () => {
  const base: ExchangeQuoteLine = {
    index: 0,
    code: "A",
    barcode: "",
    qty: -1,
    lineTotalCents: -1000,
    shelfCents: -1000,
    paidCents: 1000,
  };
  it("says nothing when the credit is the shelf price", () => {
    expect(returnedLineNotes(base)).toEqual([]);
  });
  it("says what was paid when a discount took some off", () => {
    expect(returnedLineNotes({ ...base, lineTotalCents: -800, paidCents: 800 })).toEqual([
      "Paid $8.00",
    ]);
  });
  it("explains a free item's credit", () => {
    const free = { ...base, free: true, paidCents: 0, swapAllowanceCents: 1000 };
    // Nothing bought yet: what it can still take off, not "no credit".
    expect(returnedLineNotes({ ...free, lineTotalCents: 0 })).toEqual([
      "Free item · up to $10.00 off new items",
    ]);
    expect(returnedLineNotes({ ...free, lineTotalCents: -400, swapCreditCents: 400 })).toEqual([
      "Free item · $4.00 of its $10.00 off the new items",
    ]);
    expect(returnedLineNotes({ ...free, swapCreditCents: 1000 })).toEqual([
      "Free item · $10.00 off the new items",
    ]);
    expect(returnedLineNotes({ ...free, swapAllowanceCents: 0, lineTotalCents: 0 })).toEqual([
      "Free item",
    ]);
  });
  it("explains a line charged down to nothing", () => {
    expect(
      returnedLineNotes({
        ...base,
        lineTotalCents: 0,
        chargedBackCents: 1000,
        deal: "Buy 2 get 1",
      }),
    ).toEqual(["Paid $10.00 · Buy 2 get 1 charged back $10.00"]);
  });
  it("never notes a replacement", () => {
    expect(returnedLineNotes({ ...base, qty: 1, lineTotalCents: 500 })).toEqual([]);
  });
});

describe("exchangeSummaryRows", () => {
  it("shows tax, rounding and a forfeited credit when there are any", () => {
    const q = shopFloorQuote();
    q.taxCents = 268;
    q.unroundedCents = 2701;
    q.roundingCents = 2;
    q.grandTotalCents = 2703;
    q.adjustments = [{ deal: "Buy 2 get 1", chargedBackCents: 633, forfeitedCents: 300 }];
    const rows = exchangeSummaryRows(q);
    expect(rows.map((r) => r.key)).toEqual([
      "new",
      "returned",
      "back-Buy 2 get 1",
      "forfeit-Buy 2 get 1",
      "tax",
      "rounding",
      "total",
    ]);
    expect(rows.find((r) => r.key === "forfeit-Buy 2 get 1")!.label).toBe(
      "Unused free item credit (Buy 2 get 1): $3.00",
    );
    expect(summaryAddsUp(rows)).toBe(true);
  });
  it("says what is missing, and not that a free item's credit is lost, while more is needed", () => {
    // Everything returned, nothing bought yet: the free item's credit is
    // still there to use, so it isn't "unused" yet.
    const q = shopFloorQuote();
    q.lines[0]!.lineTotalCents = 0;
    q.replacementTotalCents = 0;
    q.netCents = q.unroundedCents = -567;
    q.grandTotalCents = 0;
    q.needsMoreReplacementValue = true;
    q.shortfallCents = 567;
    q.adjustments = [{ deal: "Buy 2 get 1", chargedBackCents: 633, forfeitedCents: 900 }];
    expect(exchangeQuoteProblem(q, cart)).toBe("");
    const rows = exchangeSummaryRows(q);
    expect(rows.map((r) => r.key)).toEqual([
      "new",
      "returned",
      "back-Buy 2 get 1",
      "shortfall",
      "total",
    ]);
    expect(rows.find((r) => r.key === "shortfall")!.label).toBe("Add $5.67 more in new items");
    expect(rows.some((r) => r.label.startsWith("Unused"))).toBe(false);
    expect(summaryAddsUp(rows)).toBe(true);
  });
  it("floors the total at zero while the new items don't cover the returned ones", () => {
    const q = shopFloorQuote();
    q.replacementTotalCents = 300;
    q.lines[0]!.lineTotalCents = 300;
    q.netCents = q.unroundedCents = -267;
    q.grandTotalCents = 0;
    q.needsMoreReplacementValue = true;
    q.shortfallCents = 267;
    expect(exchangeQuoteProblem(q, cart)).toBe("");
    expect(summaryAddsUp(exchangeSummaryRows(q))).toBe(true);
  });
  it("is caught when it wouldn't add up", () => {
    const rows = exchangeSummaryRows(shopFloorQuote());
    rows[0]!.cents += 1;
    expect(summaryAddsUp(rows)).toBe(false);
    expect(summaryAddsUp(rows.filter((r) => !r.total))).toBe(false);
  });
});

// Random exchanges built the way the backend builds them: whatever they
// are, a consistent quote passes, its breakdown adds up to its total, and
// its lines add up to its net.
describe("every consistent exchange adds up on screen", () => {
  function rng(seed: number) {
    return () => {
      seed = (seed * 1103515245 + 12345) & 0x7fffffff;
      return seed / 0x7fffffff;
    };
  }
  it("for 5,000 random exchanges", () => {
    const rand = rng(42);
    const int = (n: number) => Math.floor(rand() * n);
    for (let run = 0; run < 5000; run++) {
      const lines: ExchangeQuoteLine[] = [];
      const cartLines: ExchangeCartLine[] = [];
      let back = 0;
      const n = 1 + int(6);
      for (let i = 0; i < n; i++) {
        const qty = (int(3) + 1) * (rand() < 0.5 ? -1 : 1);
        const shelf = (100 + int(20000)) * Math.abs(qty);
        if (qty < 0) {
          const free = rand() < 0.25;
          const paid = free ? 0 : shelf - int(shelf / 2);
          const swap = free ? int(shelf + 1) : 0;
          const charged = rand() < 0.3 ? int(paid + swap + 2000) : 0;
          back += charged;
          lines.push({
            index: i,
            code: `C${i}`,
            barcode: `b${i}`,
            qty,
            shelfCents: -shelf,
            paidCents: paid,
            swapCreditCents: swap,
            chargedBackCents: charged,
            swapAllowanceCents: free ? shelf : 0,
            free,
            deal: "Buy 2 get 1",
            lineTotalCents: -(paid + swap - charged),
          });
        } else {
          lines.push({
            index: i,
            code: `C${i}`,
            barcode: `b${i}`,
            qty,
            shelfCents: shelf,
            lineTotalCents: shelf - int(shelf / 3),
          });
        }
        cartLines.push({ code: `C${i}`, barcode: `b${i}`, qty });
      }
      const returned = -lines.filter((l) => l.qty < 0).reduce((s, l) => s + l.lineTotalCents, 0);
      const replacement = lines.filter((l) => l.qty > 0).reduce((s, l) => s + l.lineTotalCents, 0);
      const net = replacement - returned;
      const tax = rand() < 0.3 ? Math.round(net * 0.11) : 0;
      const rounding = rand() < 0.3 ? int(11) - 5 : 0;
      const q: ExchangeQuoteResult = {
        returnedTotalCents: returned,
        replacementTotalCents: replacement,
        netCents: net,
        taxCents: tax,
        unroundedCents: net + tax,
        roundingCents: rounding,
        grandTotalCents: Math.max(0, net + tax + rounding),
        // Short exactly by what the lines are short of, as the backend asks.
        needsMoreReplacementValue: net < 0,
        shortfallCents: Math.max(0, -net),
        lines,
        adjustments: back ? [{ deal: "Buy 2 get 1", chargedBackCents: back }] : [],
      };
      expect(exchangeQuoteProblem(q, cartLines)).toBe("");
      const rows = exchangeSummaryRows(q);
      expect(summaryAddsUp(rows)).toBe(true);
      const shown = cartLines.reduce(
        (s, _, i) => s + exchangeLineAt(q, cartLines, i)!.lineTotalCents,
        0,
      );
      expect(shown).toBe(q.netCents);
      for (const l of lines) {
        for (const note of returnedLineNotes(l)) expect(note).not.toMatch(/NaN|undefined/);
      }
      // And one cent off anywhere is caught.
      const broken = structuredClone(q);
      broken.lines[int(broken.lines.length)]!.lineTotalCents += 1;
      expect(exchangeQuoteProblem(broken, cartLines)).not.toBe("");
    }
  });
});

// Real quotes from the backend (exchange_quote_lines_test.go writes them):
// what the Go side sends is what this side checks.
describe("the backend's own quotes", () => {
  const golden: { name: string; cart: ExchangeCartLine[]; quote: unknown }[] = JSON.parse(
    readFileSync(new URL("./testdata/exchange_quotes.json", import.meta.url), "utf8"),
  );
  it("has some", () => {
    expect(golden.length).toBeGreaterThanOrEqual(5);
  });
  it.each(golden.map((g) => [g.name, g] as const))("%s adds up on screen", (_, g) => {
    const checked = checkExchangeQuote(g.quote, JSON.stringify(g.cart));
    expect(checked.error).toBe("");
    const q = checked.quote!;
    expect(summaryAddsUp(exchangeSummaryRows(q))).toBe(true);
    const shown = g.cart.reduce((s, _, i) => s + exchangeLineAt(q, g.cart, i)!.lineTotalCents, 0);
    expect(shown).toBe(q.netCents);
  });
});

describe("returnedLineTone", () => {
  const returned: ExchangeQuoteLine = {
    index: 0,
    code: "BOC5KBH15",
    barcode: "",
    qty: -1,
    lineTotalCents: -1500,
    shelfCents: -1500,
    paidCents: 1500,
  };
  it("is orange when a deal was charged back (the $15 hoodie credited $12)", () => {
    expect(
      returnedLineTone({
        ...returned,
        lineTotalCents: -1200,
        chargedBackCents: 300,
        deal: "Buy 1 Get 1",
      }),
    ).toBe("charged");
  });
  it("is green for a free item, used or not", () => {
    const free = { ...returned, free: true, paidCents: 0, swapAllowanceCents: 900 };
    expect(returnedLineTone({ ...free, lineTotalCents: 0 })).toBe("free");
    expect(returnedLineTone({ ...free, lineTotalCents: -900, swapCreditCents: 900 })).toBe("free");
  });
  it("is orange over green when a free item's credit was charged back too", () => {
    expect(
      returnedLineTone({
        ...returned,
        free: true,
        paidCents: 0,
        swapCreditCents: 500,
        chargedBackCents: 500,
        lineTotalCents: 0,
      }),
    ).toBe("charged");
  });
  it("is plain for a line credited what was paid, and never colours a new item", () => {
    expect(returnedLineTone(returned)).toBe("");
    expect(
      returnedLineTone({ ...returned, qty: 1, lineTotalCents: 1500, chargedBackCents: 300 }),
    ).toBe("");
  });
  it("colours every charged-back line of the backend's own quotes, and only those", () => {
    const golden: { quote: unknown }[] = JSON.parse(
      readFileSync(new URL("./testdata/exchange_quotes.json", import.meta.url), "utf8"),
    );
    for (const g of golden) {
      for (const l of normalizeExchangeQuote(g.quote).lines) {
        const tone = returnedLineTone(l);
        if (l.qty > 0) expect(tone).toBe("");
        else if ((l.chargedBackCents ?? 0) > 0) expect(tone).toBe("charged");
        else expect(tone).toBe(l.free ? "free" : "");
        // A coloured line always has a note saying why.
        if (tone) expect(returnedLineNotes(l)).not.toEqual([]);
      }
    }
  });
});
