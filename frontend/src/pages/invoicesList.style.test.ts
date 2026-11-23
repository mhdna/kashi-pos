// The invoice details dialog shows client, salesperson, points and note as
// plain labelled facts (no bordered cards, no coloured icons), and the
// settlement lines have no card icon.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const src = readFileSync(resolve(__dirname, "invoices-list.vue"), "utf8");
const facts = src.slice(src.indexOf('<dl class="invoice-facts'), src.indexOf("</dl>"));

describe("invoice details overview", () => {
  it("is a list of labelled facts with no icons", () => {
    for (const label of ["Client", "Salesman", "Points earned", "Note"]) expect(facts).toContain(`<dt>{{ t("${label}") }}</dt>`);
    expect(facts).not.toMatch(/<k-icon/);
    expect(src).not.toMatch(/invoice-overview-card|mdi-star-four-points-outline/);
  });

  it("settlement lines have no card icon", () => {
    expect(src).not.toMatch(/mdi-credit-card-outline" size="19"/);
  });
});

describe("sync dot column", () => {
  it("only admins get it", () => {
    expect(src).toMatch(/auth\.isAdmin \|\| header\.key !== "syncState"/);
  });
});

describe("invoice list salesperson", () => {
  it("is the name with its icon and no background fill", () => {
    const chip = src.slice(src.indexOf("#item.salespersonName"), src.indexOf("#item.settlementAccounts"));
    expect(chip).toContain('prepend-icon="mdi-account-tie"');
    expect(chip).toContain('variant="text"');
    expect(chip).not.toContain("tonal");
  });
});
