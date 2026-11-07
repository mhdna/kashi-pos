import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import rule from "./testdata/return_receipt_rule.json";
import { checkoutPrints, exchangeDoneMessage } from "./returnReceipt";

describe("checkoutPrints", () => {
  // Written by the Go test TestReturnReceiptRuleGolden from the backend's
  // own rule: the screen must say "printing" exactly when the till prints.
  it("matches the backend for every exchange", () => {
    expect(rule).toHaveLength(4);
    for (const row of rule) {
      expect(checkoutPrints("exchange", row.receiptsOn, row.tick), JSON.stringify(row)).toBe(
        row.prints,
      );
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

describe("exchangeDoneMessage", () => {
  it("says when no receipt was printed", () => {
    expect(exchangeDoneMessage("EX/CT/2026/00012", true)).toBe(
      "Exchange completed — EX/CT/2026/00012",
    );
    expect(exchangeDoneMessage("EX/CT/2026/00012", false)).toBe(
      "Exchange completed, no receipt printed — EX/CT/2026/00012",
    );
  });

  it("a printer failure wins", () => {
    expect(exchangeDoneMessage("EX/CT/2026/00012", false, "no receipt printer configured")).toBe(
      "Invoice created; receipt failed — no receipt printer configured",
    );
  });
});

// "Print return and exchange receipts" is kashi's to set (branch settings):
// the till's exchange screen has no Print receipt tick and its Settings no
// switch for it.
describe("the till has no return receipt choice of its own", () => {
  const source = (path: string) => readFileSync(resolve(process.cwd(), path), "utf8");
  it("no Print receipt tick on the exchange screen", () => {
    const home = source("src/pages/home.vue");
    expect(home).not.toMatch(/ReturnReceiptTick|label="Print receipt"/);
    // The exchange still sends the till's setting to the backend.
    expect(home).toMatch(/printReturnReceipts\.value/);
  });
  it("no Print return receipts switch in Settings", () => {
    expect(source("src/pages/settings.vue")).not.toMatch(
      /print-return-receipts|setPrintReturnReceipts/,
    );
  });
});
