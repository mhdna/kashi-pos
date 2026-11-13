import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { entryUnits, settleCashEntryCents, settledPaymentCents, type CashEntry, type PaymentEntry } from "./cashPayments";
import { roundCentsFromCurrencyUnits, type CurrencyRate, type RoundingMode } from "./money";

const LBP: CurrencyRate = { code: "LBP", symbol: "LL", unitsPerUsd: 90000, cashRoundingUnit: 10000 };
const USD: CurrencyRate = { code: "USD", symbol: "$", unitsPerUsd: 1, cashRoundingUnit: 1 };
const sum = (values: number[]) => values.reduce((a, b) => a + b, 0);

function lbpEntry(accountId: number, units: number, mode: RoundingMode = "half_up"): PaymentEntry {
  const cents = roundCentsFromCurrencyUnits(Math.abs(units), LBP, mode);
  return { accountId, amount: units, amountCents: units < 0 ? -cents : cents, currency: LBP };
}

function usdEntry(accountId: number, cents: number): PaymentEntry {
  return { accountId, amount: cents / 100, amountCents: cents, currency: USD };
}

describe("settledPaymentCents", () => {
  // SA/CT/2026/03167 at Centro: $41.56 paid with $40 and LL 200,000,
  // LL 60,000 back. It recorded $41.55.
  it("records the LL kept after change, so a $41.56 sale is paid $41.56", () => {
    const entries = [usdEntry(1, 4000), lbpEntry(2, 200000), lbpEntry(2, -60000)];
    expect(sum(entries.map((e) => e.amountCents))).toBe(4155);
    expect(settledPaymentCents(entries, "half_up")).toEqual([4000, 222, -66]);
    expect(sum(settledPaymentCents(entries, "half_up"))).toBe(4156);
  });

  it("nets each LL account on its own and leaves dollars alone", () => {
    const entries = [lbpEntry(2, 50000), usdEntry(1, 1234), lbpEntry(3, 50000), lbpEntry(2, 50000), lbpEntry(3, -5000)];
    expect(settledPaymentCents(entries, "half_up")).toEqual([56, 1234, 56, 55, -6]);
  });

  it("sends the LL of LL entries only", () => {
    expect(entryUnits(lbpEntry(2, -60000))).toBe(60000);
    expect(entryUnits(lbpEntry(2, 200000))).toBe(200000);
    expect(entryUnits(usdEntry(1, 4000))).toBeUndefined();
  });

  it("no entries, no cents", () => {
    expect(settledPaymentCents([], "half_up")).toEqual([]);
  });

  it("is one cent off nowhere: hundreds of random tills add up to the LL kept", () => {
    let seed = 1113;
    const rand = (n: number) => {
      seed = (seed * 1103515245 + 12345) % 2147483648;
      return seed % n;
    };
    for (const mode of ["half_up", "down", "up"] as RoundingMode[]) {
      for (let n = 0; n < 400; n++) {
        const entries: PaymentEntry[] = [];
        let kept = 0;
        const count = 1 + rand(5);
        for (let i = 0; i < count; i++) {
          if (i === 0 || rand(2) === 0) {
            const units = (1 + rand(200)) * 10000;
            entries.push(lbpEntry(2, units, mode));
            kept += units;
          } else {
            const units = (1 + rand(40)) * 5000;
            entries.push(lbpEntry(2, -units, mode));
            kept -= units;
          }
        }
        const cents = settledPaymentCents(entries, mode);
        const want = (kept < 0 ? -1 : 1) * roundCentsFromCurrencyUnits(Math.abs(kept), LBP, mode);
        expect(sum(cents), `${mode} ${JSON.stringify(entries.map((e) => e.amount))}`).toBe(want);
        cents.forEach((c, i) => {
          expect(Math.abs(c - entries[i]!.amountCents)).toBeLessThanOrEqual(1);
        });
      }
    }
  });
});

interface GoldenCase {
  name: string;
  mode: RoundingMode;
  unitsPerUsd: number;
  cashRoundingUnit: number;
  entries: CashEntry[];
  cents: number[];
}

describe("checkout's own LL netting (cash_payment_netting.json)", () => {
  const golden: GoldenCase[] = JSON.parse(
    readFileSync(new URL("./testdata/cash_payment_netting.json", import.meta.url), "utf8"),
  );

  it("has the cases", () => {
    expect(golden.length).toBeGreaterThan(100);
    expect(golden[0]!.name).toContain("SA/CT/2026/03167");
    expect(golden[0]!.cents).toEqual([222, -66]);
  });

  it("the calculator records every case exactly as checkout does", () => {
    for (const c of golden) {
      const currency: CurrencyRate = { code: "LBP", symbol: "LL", unitsPerUsd: c.unitsPerUsd, cashRoundingUnit: c.cashRoundingUnit };
      expect(settleCashEntryCents(c.entries, currency, c.mode), c.name).toEqual(c.cents);
    }
  });

  it("a cent off anywhere is caught", () => {
    const c = golden[0]!;
    const currency: CurrencyRate = { code: "LBP", symbol: "LL", unitsPerUsd: c.unitsPerUsd, cashRoundingUnit: c.cashRoundingUnit };
    const got = settleCashEntryCents(c.entries, currency, c.mode);
    got[1] = (got[1] ?? 0) - 1;
    expect(got).not.toEqual(c.cents);
  });
});
