import { describe, expect, it } from "vitest";
import rule from "./testdata/return_receipt_rule.json";
import { checkoutPrints, exchangeDoneMessage, startingReturnTick } from "./returnReceipt";

describe("checkoutPrints", () => {
  // Written by the Go test TestReturnReceiptRuleGolden from the backend's
  // own rule: the screen must say "printing" exactly when the till prints.
  it("matches the backend for every exchange", () => {
    expect(rule).toHaveLength(4);
    for (const row of rule) {
      expect(checkoutPrints("exchange", row.receiptsOn, row.tick), JSON.stringify(row)).toBe(row.prints);
    }
  });

  it("a sale prints whenever receipts are on, whatever the return tick", () => {
    for (const tick of [true, false]) {
      expect(checkoutPrints("sale", true, tick)).toBe(true);
      expect(checkoutPrints("sale", false, tick)).toBe(false);
    }
  });

  it("an unticked exchange never prints", () => {
    expect(checkoutPrints("exchange", true, false)).toBe(false);
  });
});

describe("startingReturnTick", () => {
  it("starts where the setting is", () => {
    expect(startingReturnTick(true)).toBe(true);
    expect(startingReturnTick(false)).toBe(false);
  });
});

describe("exchangeDoneMessage", () => {
  it("says when no receipt was printed", () => {
    expect(exchangeDoneMessage("EX/CT/2026/00012", true)).toBe("Exchange completed — EX/CT/2026/00012");
    expect(exchangeDoneMessage("EX/CT/2026/00012", false)).toBe("Exchange completed, no receipt printed — EX/CT/2026/00012");
  });

  it("a printer failure wins", () => {
    expect(exchangeDoneMessage("EX/CT/2026/00012", false, "no receipt printer configured")).toBe(
      "Invoice created; receipt failed — no receipt printer configured",
    );
  });
});
