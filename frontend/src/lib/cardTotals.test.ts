import { describe, expect, it } from "vitest";
import { discountLabel, invoiceTotalLabel, type CardTotalsInput } from "./cardTotals";

const base: CardTotalsInput = {
  quoteFailed: false,
  exchangeMode: false,
  hasItems: true,
  totalCents: 25000,
  diffCents: -25000,
};

describe("invoiceTotalLabel", () => {
  it("is not shown while the big number is the total (nothing paid yet)", () => {
    expect(invoiceTotalLabel(base)).toBe("");
  });

  it("is shown once part is paid: the card says what is left, the label the total", () => {
    // $250 sale, $130 paid: $120 left to pay
    expect(invoiceTotalLabel({ ...base, diffCents: -12000 })).toBe("Total $250.00");
  });

  it("is shown when it is paid in full ('Paid' is not the total)", () => {
    expect(invoiceTotalLabel({ ...base, diffCents: 0 })).toBe("Total $250.00");
  });

  it("is shown when there is change to give back", () => {
    expect(invoiceTotalLabel({ ...base, diffCents: 500 })).toBe("Total $250.00");
  });

  it("is not shown when the change equals the total (it is the same number)", () => {
    expect(invoiceTotalLabel({ ...base, diffCents: 25000 })).toBe("");
  });

  it("is shown for a settlement change that starts from what was paid", () => {
    // Change settlement on a $250 invoice: $120 left to pay of $250.
    expect(
      invoiceTotalLabel({ ...base, hasItems: false, settlementInvoiceId: 7, diffCents: -12000 }),
    ).toBe("Total $250.00");
  });

  it("is never shown for a failed quote, an exchange, an empty cart or a zero total", () => {
    const paid = { ...base, diffCents: -12000 };
    expect(invoiceTotalLabel({ ...paid, quoteFailed: true })).toBe("");
    expect(invoiceTotalLabel({ ...paid, exchangeMode: true })).toBe("");
    expect(invoiceTotalLabel({ ...paid, hasItems: false })).toBe("");
    expect(invoiceTotalLabel({ ...paid, totalCents: 0, diffCents: 0 })).toBe("");
  });

  it("formats cents exactly (a cent off anywhere is caught)", () => {
    expect(invoiceTotalLabel({ ...base, totalCents: 12856, diffCents: 0 })).toBe("Total $128.56");
    expect(invoiceTotalLabel({ ...base, totalCents: 5, diffCents: 0 })).toBe("Total $0.05");
    expect(invoiceTotalLabel({ ...base, totalCents: 100001, diffCents: 0 })).toBe("Total $1,000.01");
  });

  it("randomized: shown exactly when the big number (|diff|) differs from the total", () => {
    let seed = 12345;
    const rand = (n: number) => {
      seed = (seed * 1103515245 + 12345) & 0x7fffffff;
      return seed % n;
    };
    for (let i = 0; i < 500; i++) {
      const total = 1 + rand(500000);
      const diff = [0, -total, total, -rand(total + 1), rand(total + 1)][rand(5)]!;
      const label = invoiceTotalLabel({ ...base, totalCents: total, diffCents: diff });
      const bigNumberIsTotal = Math.abs(diff) === total;
      expect(label !== "").toBe(!bigNumberIsTotal);
      if (label) expect(label).toBe(`Total ${new Intl.NumberFormat("en-US", { style: "currency", currency: "USD" }).format(total / 100)}`);
    }
  });
});

describe("discountLabel", () => {
  it("is empty for no discount", () => {
    expect(discountLabel(undefined)).toBe("");
    expect(discountLabel(0)).toBe("");
    expect(discountLabel(-5)).toBe("");
    expect(discountLabel(NaN)).toBe("");
  });

  it("names the amount taken off, exactly", () => {
    expect(discountLabel(4200)).toBe("Discount −$42.00");
    expect(discountLabel(1)).toBe("Discount −$0.01");
    expect(discountLabel(123456)).toBe("Discount −$1,234.56");
  });
});
