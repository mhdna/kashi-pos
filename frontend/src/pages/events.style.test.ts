// The Coming up list: each event's icon and its "In 7 days" are coloured, with
// no tinted circle or pill behind them.
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

  it("still colours the icon and the when-label with the event's colour", () => {
    expect(upcoming).toMatch(/<k-icon :icon="e.icon" :color="e.color"/);
    expect(upcoming).toMatch(/variant="text" :color="e.color"/);
  });
});
