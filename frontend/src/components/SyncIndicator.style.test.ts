// The sync light does not move: no breathing ring, no drifting or spinning
// icon, while it works.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const src = readFileSync(resolve(__dirname, "SyncIndicator.vue"), "utf8");
const css = src.slice(src.indexOf("<style")).replace(/\/\*[\s\S]*?\*\//g, "");

describe("sync light", () => {
  it("has no animation or keyframes", () => {
    expect(css).not.toMatch(/animation|@keyframes/);
  });
  it("draws no ring around the busy icon", () => {
    expect(css).not.toMatch(/sync-status-icon--busy::after/);
  });
});
