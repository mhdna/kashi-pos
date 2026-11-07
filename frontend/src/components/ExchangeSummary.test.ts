import { readFileSync } from "node:fs";
import { createSSRApp, h } from "vue";
import { renderToString } from "vue/server-renderer";
import { describe, expect, it } from "vitest";
import ExchangeSummary from "./ExchangeSummary.vue";
import { checkExchangeQuote, type ExchangeCartLine } from "@/lib/exchangeQuote";

// The rows as the cashier reads them: [label, amount or "" for a note].
async function rendered(quote: unknown): Promise<[string, string][]> {
  const html = await renderToString(
    createSSRApp({ render: () => h(ExchangeSummary, { quote: quote as any }) }),
  );
  const rows: [string, string][] = [];
  const rowRe =
    /<div[^>]*class="[^"]*exchange-summary-row\b[^"]*"[^>]*data-row="[^"]*"[^>]*>([\s\S]*?)<\/div>/g;
  for (const m of html.matchAll(rowRe)) {
    const inner = m[1] ?? "";
    const label = /exchange-summary-label"[^>]*>([^<]*)</.exec(inner)?.[1] ?? "";
    const amount = /exchange-summary-amount"[^>]*>([^<]*)</.exec(inner)?.[1] ?? "";
    rows.push([label.replace(/&#39;/g, "'").replace(/&amp;/g, "&"), amount]);
  }
  return rows;
}

const parse = (text: string) =>
  (text.startsWith("−") ? -1 : 1) * Math.round(Number(text.replace(/[−$,]/g, "")) * 100);

const golden: { name: string; cart: ExchangeCartLine[]; quote: unknown }[] = JSON.parse(
  readFileSync(new URL("../lib/testdata/exchange_quotes.json", import.meta.url), "utf8"),
);
const quoteNamed = (name: string) => {
  const g = golden.find((x) => x.name === name)!;
  return checkExchangeQuote(g.quote, JSON.stringify(g.cart)).quote!;
};

describe("ExchangeSummary", () => {
  it("shows the shop-floor exchange so it can be read out to the customer", async () => {
    expect(await rendered(quoteNamed("paid deal unit for an unknown item"))).toEqual([
      ["New items", "$120.00"],
      ["Returned credit", "−$30.00"],
      ["Buy 2 get 1 no longer applies: $50.00 charged back", ""],
      ["To pay", "$90.00"],
    ]);
  });

  it("shows tax and cash rounding as their own rows", async () => {
    expect(await rendered(quoteNamed("tax and cash rounding"))).toEqual([
      ["New items", "$100.00"],
      ["Returned credit", "−$20.00"],
      ["Tax", "$8.80"],
      ["Cash rounding", "$0.03"],
      ["To pay", "$88.83"],
    ]);
  });

  it("says what is missing when the new items don't cover the returned ones", async () => {
    const rows = await rendered(quoteNamed("not enough new items yet"));
    expect(rows.at(-1)).toEqual(["To pay", "$0.00"]);
    expect(rows.map((r) => r[0])).toContain("Add $80.00 more in new items");
  });

  // Whatever the backend sends, the amounts on screen add up to "To pay".
  it.each(golden.map((g) => [g.name] as const))(
    "%s: the rows on screen add up to To pay",
    async (name) => {
      const rows = await rendered(quoteNamed(name));
      const total = rows.find((r) => r[0] === "To pay")!;
      const sum = rows
        .filter((r) => r[1] && r[0] !== "To pay")
        .reduce((s, r) => s + parse(r[1]), 0);
      expect(Math.max(0, sum)).toBe(parse(total[1]));
    },
  );
});
