import { describe, expect, it } from "vitest";
import { barcodeFocusBlocked } from "./barcodeFocus";

describe("barcodeFocusBlocked", () => {
  it("the reported case: searching in place keeps the focus even after a dialog closes", () => {
    // A number pad or item dialog closing sets the shared flag false mid-search.
    expect(barcodeFocusBlocked({ searchingInPlace: true, dialogOpen: false, overlayOpen: false })).toBe(true);
  });

  it("nothing open: the barcode field may take the focus", () => {
    expect(barcodeFocusBlocked({ searchingInPlace: false, dialogOpen: false, overlayOpen: false })).toBe(false);
  });

  it("every combination: blocked whenever any one of them is on", () => {
    for (let n = 0; n < 8; n++) {
      const s = { searchingInPlace: !!(n & 1), dialogOpen: !!(n & 2), overlayOpen: !!(n & 4) };
      expect(barcodeFocusBlocked(s)).toBe(n !== 0);
    }
  });
});
