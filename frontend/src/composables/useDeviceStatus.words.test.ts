// The devices dialog uses short words for a device it can't see yet.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const src = readFileSync(resolve(__dirname, "useDeviceStatus.ts"), "utf8");

describe("device status words", () => {
  it("are short", () => {
    expect(src).toContain('unknown: "Unknown"');
    expect(src).toContain('unknown: "Scan to detect"');
    expect(src).toContain('unknown: "Not set"');
    expect(src).not.toMatch(/Not known yet|so the till knows it|None chosen in Settings/);
  });
});
