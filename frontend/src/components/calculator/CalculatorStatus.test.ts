// @vitest-environment happy-dom
//
// The calculator's total card: the line under the total is the cash split when
// that says something the total doesn't, otherwise the same total in the other
// currency; only a one-currency till has no second line.
import { describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import ui from "@/ui";

vi.mock("@/lib/display", async () => {
  const { ref } = await import("vue");
  const calculatorTotalCurrency = ref("USD");
  return {
    calculatorTotalCurrency,
    setCalculatorTotalCurrency: (code: string) => (calculatorTotalCurrency.value = code),
  };
});

import CalculatorStatus from "./CalculatorStatus.vue";
import { calculatorTotalCurrency } from "@/lib/display";

const BOTH = [
  { code: "USD", text: "$30.00", rounded: false },
  { code: "LBP", text: "LL 2,685,000", rounded: false },
];

function mountStatus(
  remaining: { first: string; second: string; firstCode?: string },
  amountsByCurrency = BOTH,
  extra: Record<string, unknown> = {},
) {
  return mount(CalculatorStatus, {
    global: { plugins: [ui] },
    props: {
      paymentStatus: "Remaining",
      statusColor: "calculator-green",
      showReadyState: false,
      remainingAmount: {
        ...remaining,
        fullPrimary: "$30.00",
        fullSecondary: "LL 2,685,000",
        secondaryRounded: false,
        amountsByCurrency,
      },
      ...extra,
    },
  });
}

describe("CalculatorStatus split line", () => {
  it("shows $30.00 once when the split is just $30, with the LL amount under it", () => {
    const w = mountStatus({ first: "$30", second: "", firstCode: "USD" });
    expect(w.find(".calculator-total-usd").text()).toBe("$30.00");
    // No repeated "$30": the other currency's total instead.
    expect(w.find(".calculator-amounts-other").text()).toBe("LL 2,685,000");
    expect(w.text()).not.toContain("$30 ");
    expect(w.find(".calculator-total-usd").classes()).not.toContain("calculator-total-alone");
  });

  it("shows the split when it has two halves", () => {
    const w = mountStatus({ first: "$30", second: "LL 45,000", firstCode: "" });
    expect(w.find(".calculator-amounts-split").text()).toBe("$30+LL 45,000");
    expect(w.find(".calculator-amounts-other").exists()).toBe(false);
  });

  it("shows a lone LBP split under a USD total", () => {
    const w = mountStatus({ first: "LL 450,000", second: "", firstCode: "LBP" });
    expect(w.find(".calculator-amounts-split").text()).toBe("LL 450,000");
  });

  it("shows a lone $30 split when the total is flipped to LBP", async () => {
    calculatorTotalCurrency.value = "LBP";
    const w = mountStatus({ first: "$30", second: "", firstCode: "USD" });
    expect(w.find(".calculator-total-usd").text()).toBe("LL 2,685,000");
    expect(w.find(".calculator-amounts-split").text()).toBe("$30");
    expect(w.find(".calculator-amounts-other").exists()).toBe(false); // the split already says it
    calculatorTotalCurrency.value = "USD";
  });

  it("hides the line on a USD-only till (no split currency) and centres the total", () => {
    const w = mountStatus({ first: "$30.00", second: "", firstCode: "USD" }, [{ code: "USD", text: "$30.00", rounded: false }]);
    expect(w.find(".calculator-amounts-split").exists()).toBe(false);
    expect(w.find(".calculator-amounts-other").exists()).toBe(false);
    expect(w.find(".calculator-total-usd").classes()).toContain("calculator-total-alone");
  });

  it("flipping the total to LL puts the dollars under it, and back again", async () => {
    // LL first: the total is LL, the split line says $30.
    calculatorTotalCurrency.value = "LBP";
    const flipped = mountStatus({ first: "$30", second: "", firstCode: "USD" });
    expect(flipped.find(".calculator-total-usd").text()).toBe("LL 2,685,000");
    expect(flipped.find(".calculator-amounts-split").text()).toBe("$30");
    calculatorTotalCurrency.value = "USD";
    // USD: the total is $30.00 and LL sits under it: a line on both views.
    const usd = mountStatus({ first: "$30", second: "", firstCode: "USD" });
    expect(usd.findAll(".calculator-amounts-split")).toHaveLength(1);
    expect(usd.find(".calculator-amounts-split").text()).toBe("LL 2,685,000");
  });
});

describe("CalculatorStatus total and discount", () => {
  const one = { first: "$30", second: "", firstCode: "USD" };

  it("shows the invoice total in the corner only when it is given", () => {
    expect(mountStatus(one).find("[data-test=invoice-total]").exists()).toBe(false);
    const w = mountStatus(one, BOTH, { totalText: "Total $250.00" });
    expect(w.find("[data-test=invoice-total]").text()).toBe("Total $250.00");
  });

  it("keeps the total on the card when it is paid in full", () => {
    const w = mountStatus(one, BOTH, { showReadyState: true, paymentStatus: "Paid", totalText: "Total $250.00" });
    expect(w.text()).toContain("Paid");
    expect(w.find("[data-test=invoice-total]").text()).toBe("Total $250.00");
  });

  it("never shows it over an error or an exchange shortfall", () => {
    const w = mountStatus(one, BOTH, { quoteError: "boom", totalText: "" });
    expect(w.find("[data-test=invoice-total]").exists()).toBe(false);
  });

  it("shows the discount taken off, and no pill without one", () => {
    expect(mountStatus(one).find("[data-test=discount-total]").exists()).toBe(false);
    const w = mountStatus(one, BOTH, { discountText: "Discount −$34.50" });
    expect(w.find("[data-test=discount-total]").text()).toBe("Discount −$34.50");
  });

  it("stacks the total, Rounded and the discounts down the left side, one under the other", () => {
    const w = mountStatus({ first: "$128", second: "LL 50,000", firstCode: "" }, BOTH, {
      totalText: "Total $128.56",
      roundedFromText: "Rounded from $128.56",
      invoiceDiscountText: "15% over $150 −$42",
      discountText: "Discount −$42.00",
    });
    const left = w.find("[data-test=card-left]");
    // One column holding them all, in this order (nothing floats on the right).
    expect(left.findAll(":scope > *").map((el) => el.text())).toEqual([
      "Total $128.56",
      "Rounded",
      "15% over $150 −$42",
      "Discount −$42.00",
    ]);
    expect(w.findAll(".calculator-amounts-discount")).toHaveLength(2);
    // The total is a chip, like the others.
    expect(w.find("[data-test=invoice-total]").classes()).toContain("calculator-amounts-total");
  });

  it("the card keeps its own height whatever is on the left (no per-chip sizing)", () => {
    for (const extra of [
      {},
      { totalText: "Total $250.00" },
      { totalText: "Total $250.00", discountText: "Discount −$5.00", roundedFromText: "Rounded from $1", invoiceDiscountText: "10% −$2" },
    ]) {
      const card = mountStatus(one, BOTH, extra).find(".calculator-amounts");
      expect(card.attributes("style") ?? "").not.toMatch(/height|left-items/);
    }
  });


  it("shows only the Total over Paid, an error or an exchange hint: no pills there", () => {
    const pills = { roundedFromText: "Rounded from $1", discountText: "Discount −$5.00", totalText: "Total $250.00" };
    const paid = mountStatus(one, BOTH, { ...pills, showReadyState: true, paymentStatus: "Paid" });
    expect(paid.find("[data-test=card-left]").text()).toBe("Total $250.00");
    const failed = mountStatus(one, BOTH, { ...pills, quoteError: "boom" });
    expect(failed.find("[data-test=discount-total]").exists()).toBe(false);
  });

  it("has nothing on the left when there is nothing to say", () => {
    expect(mountStatus(one).find("[data-test=card-left]").exists()).toBe(false);
  });
});
