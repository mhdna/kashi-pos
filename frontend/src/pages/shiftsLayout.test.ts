import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

// Vitest doesn't load component CSS, so the rules are read from the source.
const css = readFileSync(resolve(process.cwd(), "src/pages/shifts.vue"), "utf8");

describe("Shifts page layout", () => {
  it("the strip between the left card and the right column is 4px, not 12px", () => {
    expect(css).toMatch(/\.shifts-cards \{[^}]*gap: 4px/);
    expect(css).not.toMatch(/\.shifts-cards \{[^}]*gap: 12px/);
  });
  it("the two columns still share the width and can shrink", () => {
    expect(css).toMatch(/\.shifts-right \{[^}]*flex: 2/);
    expect(css).toMatch(/\.shifts-right \{[^}]*min-width: 0/);
  });
});
