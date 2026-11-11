// @vitest-environment happy-dom
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

// An icon placed in the middle of a ring was rotated -90 degrees along with
// the ring (the rule hit every svg inside), so the sync light's download
// arrow showed sideways. The ring's own svg is the only one that may rotate.
describe("progress ring css", () => {
  const css = readFileSync(resolve(__dirname, "ui.css"), "utf8");
  it("rotates only the ring's own svg, not an icon in its slot", () => {
    const rule = css.split("\n").find((l) => l.includes("rotate(-90deg)") && l.includes(".k-progress-circular"));
    expect(rule).toBeTruthy();
    expect(rule).toContain(".k-progress-circular > svg");
    expect(rule).not.toMatch(/\.k-progress-circular svg/);
  });
});
