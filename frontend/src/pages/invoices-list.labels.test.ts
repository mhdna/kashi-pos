// The invoice details list how a sale was paid without a "Settlement" heading
// over them: the rows and "Total paid" say it.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const src = readFileSync(resolve(__dirname, "invoices-list.vue"), "utf8");

describe("invoice details payments", () => {
  it("have no Settlement heading, and still their Total paid row", () => {
    expect(src).not.toMatch(/section-label">\s*Settlement\s*</);
    expect(src).toContain("Total paid");
  });
});
