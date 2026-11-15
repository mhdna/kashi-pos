// The unknown-items QR card gives its instruction in Arabic only: the English
// line that repeated it is gone, the title stays.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const src = readFileSync(resolve(__dirname, "UnknownPicturesCorner.vue"), "utf8");

describe("unknown items QR card", () => {
  it("has no English copy of the Arabic instruction", () => {
    expect(src).not.toMatch(/Scan this code with the phone|Scan, then photograph each item/);
  });
  it("keeps the Arabic instruction and the title", () => {
    expect(src).toMatch(/امسح الرمز والتقط صورة/);
    expect(src).toMatch(/Unknown items need a picture/);
  });
});
