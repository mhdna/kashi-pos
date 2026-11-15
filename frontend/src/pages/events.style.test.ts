// The Coming up list: each event's icon and its "In 7 days" are plain neutral
// grey, with no tinted circle or pill behind them.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const src = readFileSync(resolve(__dirname, "events.vue"), "utf8");
const upcoming = src.slice(src.indexOf('<section class="upcoming">'), src.indexOf('<section class="month">'));

describe("events page, Coming up list", () => {
  it("has no tonal avatar or tonal chip", () => {
    expect(upcoming).not.toMatch(/variant="tonal"/);
    expect(upcoming).not.toMatch(/<k-avatar/);
  });

  it("shows the icon and the when-label in neutral grey, not the event's colour", () => {
    expect(upcoming).toMatch(/<k-icon :icon="e.icon" size="24" \/>/);
    expect(upcoming).not.toMatch(/:color="e.color"/);
    expect(upcoming).toMatch(/<span class="upcoming-when">/);
  });

  it("writes a date range with 'to', not a dash", () => {
    expect(src).not.toMatch(/\$\{fmt\(e\.start\)\} [–—] /);
    expect(src).toMatch(/\$\{fmt\(e\.start\)\} to \$\{fmt\(e\.end\)\}/);
  });
});
