// Password fields show one eye (the app's), not the webview's native one next to it.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const css = readFileSync(resolve(__dirname, "compact.scss"), "utf8");

describe("native password reveal", () => {
  it("is hidden so only the app's eye shows", () => {
    expect(css).toMatch(/input::-ms-reveal,\s*input::-ms-clear\s*\{\s*display:\s*none;/);
  });
  it("compact.scss is loaded globally", () => {
    expect(readFileSync(resolve(__dirname, "../main.ts"), "utf8")).toMatch(/styles\/compact\.scss/);
  });
});
