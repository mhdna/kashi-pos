// The keyboard's C key is neutral grey like backspace, not red.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const src = readFileSync(resolve(__dirname, "InlineKeyboard.vue"), "utf8");

describe("inline keyboard clear key", () => {
  it("shares the backspace key's neutral background", () => {
    expect(src).toMatch(/\.inline-keyboard-clear,\s*\.inline-keyboard-backspace \{\s*background-color: rgba\(var\(--k-theme-on-surface\), 0\.1\);/);
    expect(src).not.toMatch(/#c43f3f/);
  });
});
