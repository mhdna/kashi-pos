import { describe, expect, it } from "vitest";
import { splitRepeatsTotal } from "./totalSplit";

describe("splitRepeatsTotal", () => {
  it("hides a lone $30 under a $30.00 total", () => {
    expect(splitRepeatsTotal({ first: "$30", second: "", firstCode: "USD" }, "USD")).toBe(true);
  });

  it("keeps a two-part split", () => {
    expect(splitRepeatsTotal({ first: "$30", second: "LL 45,000", firstCode: "USD" }, "USD")).toBe(false);
  });

  it("keeps a lone amount in another currency than the total (it is a conversion)", () => {
    expect(splitRepeatsTotal({ first: "LL 540,000", second: "", firstCode: "LBP" }, "USD")).toBe(false);
    expect(splitRepeatsTotal({ first: "$30", second: "", firstCode: "USD" }, "LBP")).toBe(false);
  });

  it("hides a lone LBP amount under an LBP total", () => {
    expect(splitRepeatsTotal({ first: "LL 540,000", second: "", firstCode: "LBP" }, "LBP")).toBe(true);
  });

  it("keeps the line when the currency is unknown", () => {
    expect(splitRepeatsTotal({ first: "$30", second: "" }, "USD")).toBe(false);
    expect(splitRepeatsTotal({ first: "$30", second: "", firstCode: "USD" }, "")).toBe(false);
  });
});
