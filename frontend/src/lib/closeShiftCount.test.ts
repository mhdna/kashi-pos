import { describe, expect, it } from "vitest";
import { autofillCounts, countValid, fieldCents, usdDenominations } from "./closeShiftCount";

const noBills = Object.fromEntries(usdDenominations.map((d) => [d, ""])) as Record<number, string>;

describe("close-shift autofill", () => {
  it("keeps a negative LBP (more change went out than came in)", () => {
    const f = autofillCounts({ expectedLbp: -686000, expectedVisa: 0, expectedWhish: 0 });
    expect(f.lbp).toBe("-686000");
  });

  it("keeps negative Visa and Whish to the cent", () => {
    const f = autofillCounts({ expectedLbp: 0, expectedVisa: -1250, expectedWhish: -1 });
    expect(f.visa).toBe("-12.50");
    expect(f.whish).toBe("-0.01");
    expect(fieldCents(f.visa)).toBe(-1250);
    expect(fieldCents(f.whish)).toBe(-1);
  });

  it("fills positive amounts and bill counts as before", () => {
    const f = autofillCounts({ expectedLbp: 4500000, expectedVisa: 12000, expectedWhish: 800, usdBillCounts: { "100": 2, "5": 1 } });
    expect(f).toEqual({ lbp: "4500000", visa: "120.00", whish: "8.00", bills: { ...noBills, 100: "2", 5: "1" } });
  });

  it("missing totals fill 0, not blank", () => {
    expect(autofillCounts({})).toEqual({ lbp: "0", visa: "0.00", whish: "0.00", bills: noBills });
  });

  it("random totals: what autofill fills is exactly the expected, and one off is caught", () => {
    let seed = 7;
    const rand = (n: number) => {
      seed = (seed * 1103515245 + 12345) % 2 ** 31;
      return seed % n;
    };
    for (let run = 0; run < 500; run++) {
      const lbp = rand(20_000_000) - 10_000_000;
      const visa = rand(200_000) - 100_000;
      const whish = rand(200_000) - 100_000;
      const f = autofillCounts({ expectedLbp: lbp, expectedVisa: visa, expectedWhish: whish });
      expect(Number(f.lbp)).toBe(lbp);
      expect(fieldCents(f.visa)).toBe(visa);
      expect(fieldCents(f.whish)).toBe(whish);
      expect(countValid(f)).toBe(true);
      expect(fieldCents(((visa - 1) / 100).toFixed(2))).not.toBe(visa);
      expect(Number(String(lbp - 1))).not.toBe(lbp);
    }
  });
});

describe("close-shift count validity", () => {
  const base = { lbp: "0", visa: "0.00", whish: "0.00", bills: noBills };

  it("accepts negative LBP, Visa and Whish", () => {
    expect(countValid({ ...base, lbp: "-686000" })).toBe(true);
    expect(countValid({ ...base, visa: "-12.50", whish: "-3.00" })).toBe(true);
  });

  it("needs an LBP count", () => {
    expect(countValid({ ...base, lbp: "" })).toBe(false);
    expect(countValid({ ...base, lbp: "-" })).toBe(false);
    expect(countValid({ ...base, lbp: "1.5" })).toBe(false);
  });

  it("a bill count is never negative or a fraction", () => {
    expect(countValid({ ...base, bills: { ...noBills, 20: "-1" } })).toBe(false);
    expect(countValid({ ...base, bills: { ...noBills, 20: "1.5" } })).toBe(false);
    expect(countValid({ ...base, bills: { ...noBills, 20: "3" } })).toBe(true);
  });
});
