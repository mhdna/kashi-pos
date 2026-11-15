// After checkout the till goes straight back to the sale screen; the
// "printed" notification is enough, so there's no "Done! :)" card.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const src = readFileSync(resolve(__dirname, "home.vue"), "utf8");

describe("after checkout", () => {
  it("shows no Done card or smiley", () => {
    expect(src).not.toMatch(/Done!|print-face|printPhase/);
  });
});
