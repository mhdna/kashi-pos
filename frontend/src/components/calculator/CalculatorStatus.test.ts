// @vitest-environment happy-dom
//
// The calculator's total card: the split line under the total only shows
// when it says something the total doesn't.
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

function mountStatus(remaining: { first: string; second: string; firstCode?: string }, amountsByCurrency = BOTH) {
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
    },
  });
}

describe("CalculatorStatus split line", () => {
  it("shows $30.00 once when the split is just $30", () => {
    const w = mountStatus({ first: "$30", second: "", firstCode: "USD" });
    expect(w.find(".calculator-total-usd").text()).toBe("$30.00");
    expect(w.find(".calculator-amounts-split").exists()).toBe(false);
  });

  it("shows the split when it has two halves", () => {
    const w = mountStatus({ first: "$30", second: "LL 45,000", firstCode: "" });
    expect(w.find(".calculator-amounts-split").text()).toBe("$30+LL 45,000");
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
    calculatorTotalCurrency.value = "USD";
  });

  it("hides the line on a USD-only till (no split currency)", () => {
    const w = mountStatus({ first: "$30.00", second: "", firstCode: "USD" }, [{ code: "USD", text: "$30.00", rounded: false }]);
    expect(w.find(".calculator-amounts-split").exists()).toBe(false);
  });
});
