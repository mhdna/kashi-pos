// The manager password screens (the Stats page and the shift income dialog)
// keep their centred title, field and Open button, with no lock badge; the nav
// calls the page Stats with a chart icon.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const read = (p: string) => readFileSync(resolve(__dirname, p), "utf8");
const page = read("conversion.vue");
const shift = read("../components/shifts/ShiftIncomeCard.vue");
const nav = read("../components/NavDrawer.vue");

describe("manager password screens", () => {
  it("the Stats page unlock is the centred Manager password form, no lock", () => {
    const block = page.slice(page.indexOf('<div v-if="!unlocked"'), page.indexOf('<template v-else-if="report">'));
    expect(block).toMatch(/<h2 class="unlock-title">Manager password<\/h2>/);
    expect(block).not.toMatch(/mdi-lock|unlock-badge/);
    expect(page).toMatch(/\.unlock-card \{[^}]*text-align: center/);
  });

  it("the shift income manager dialog has no lock badge or extra-round card", () => {
    const block = shift.slice(shift.indexOf('<k-dialog v-model="managerDialog.show"'), shift.indexOf("</k-dialog>", shift.indexOf('<k-dialog v-model="managerDialog.show"')));
    expect(block).not.toMatch(/mdi-lock|manager-dialog-badge|rounded="xl"/);
    expect(block).toMatch(/Manager password/);
  });
});

describe("nav", () => {
  it("names the conversion page Stats, with a chart icon", () => {
    expect(nav).toMatch(/to: "\/conversion",\s*icon: "mdi-chart-bar",\s*t: "Stats",/);
    expect(nav).not.toMatch(/t: "Conversion"/);
  });
});

describe("stats page surfaces", () => {
  it("the headline card, tile icons and buy rates have no tinted background", () => {
    expect(page).toMatch(/<k-card class="headline[^"]*" rounded="lg" variant="outlined">/);
    expect(page).not.toMatch(/<k-avatar/);
    expect(page).not.toMatch(/<k-chip/);
  });
});

describe("day table header", () => {
  it("has plain labels, no icons", () => {
    const head = page.slice(page.indexOf('<div class="day day--head">'), page.indexOf('<div v-for="day in shownDays"'));
    expect(head).toContain("Visitors");
    expect(head).not.toMatch(/<k-icon/);
  });
});

describe("headline wording", () => {
  it("is short: Buy rate, then sales and visitors", () => {
    expect(page).not.toMatch(/of every 100 visitors buy/);
    expect(page).toMatch(/>\s*Buy rate\s*<\/div>/);
  });
});
