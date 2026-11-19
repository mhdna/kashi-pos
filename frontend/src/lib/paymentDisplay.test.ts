import { describe, expect, it } from "vitest";
import { lbpLabel, paymentLabels, usdLabel } from "./paymentDisplay";

describe("paymentLabels", () => {
  it("the reported case: LBP given back reads -60,000 L.L., with its dollars beside it", () => {
    expect(paymentLabels({ amount: -67, currencyCode: "LBP", lbpAmount: -60_000 })).toEqual({ main: "-60,000 L.L.", equivalent: "-$0.67" });
  });
  it("dollars stay dollars; an LBP payment without a rate stays in dollars", () => {
    expect(paymentLabels({ amount: 2100, currencyCode: "USD" })).toEqual({ main: "$21.00", equivalent: "" });
    expect(paymentLabels({ amount: 335, currencyCode: "LBP", lbpAmount: null })).toEqual({ main: "$3.35", equivalent: "" });
  });
  it("formats", () => {
    expect(lbpLabel(300_000)).toBe("300,000 L.L.");
    expect(lbpLabel(0)).toBe("0 L.L.");
    expect(usdLabel(-5)).toBe("-$0.05");
    expect(usdLabel(undefined)).toBe("$0.00");
  });
});
