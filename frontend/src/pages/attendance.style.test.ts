// The attendance page: a one-line machine status in green or red text (no
// tinted card, no smiley face), IN and OUT as green and red text with no
// boxes, and no dashes or middle dots in its text.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const src = readFileSync(resolve(__dirname, "attendance.vue"), "utf8");

describe("attendance page", () => {
  it("shows the machine status as short colored text, no card or face", () => {
    expect(src).toContain('"Machine connected"');
    expect(src).toContain('"Machine not connected"');
    expect(src).not.toMatch(/status-face|status-card|":\)"/);
    expect(src).toContain("Check the cable.");
    expect(src).not.toMatch(/tell your manager/i);
  });

  it("IN and OUT are green and red text with no background", () => {
    const css = src.slice(src.indexOf(".attendance-event {"), src.indexOf(".attendance-event__time"));
    expect(css).not.toMatch(/background|border/);
    expect(css).toMatch(/--in \.attendance-event__label \{\s*color: #2e7d32/);
    expect(css).toMatch(/--out \.attendance-event__label \{\s*color: #c62828/);
  });

  it("has no long dashes or middle dots", () => {
    const template = src.slice(0, src.indexOf("<script"));
    expect(template).not.toMatch(/[—–·]/);
    expect(src).not.toMatch(/` [^`]*(·| -- )[^`]*`|\$\{[^}]*\} (·|--) /);
  });

  it("employee avatars show two initials, like the salesperson tiles", () => {
    expect(src).toMatch(/<InitialsAvatar\s+:name="item.employeeName/);
    expect(src).not.toMatch(/employeeName\?\.charAt\(0\)/);
  });
});
