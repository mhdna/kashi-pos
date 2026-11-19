import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

// The cart header (Salesperson, Client, Rewards, Referral) fits beside the
// invoice number on a till's screen: narrow buttons, small labels, small gaps.
describe("cart header stays narrow", () => {
  const read = (f: string) => readFileSync(resolve(__dirname, f), "utf8");
  it("Salesperson and Client buttons are 92px", () => {
    for (const f of ["ClientButton.vue", "SalespersonsDialog.vue"]) {
      expect(read(f), f).toMatch(/\.header-fixed-width-btn \{\s*width: 92px;/);
    }
  });
  it("labels above the buttons are small, and the row's gaps tight", () => {
    for (const f of ["ClientButton.vue", "SalespersonsDialog.vue", "RewardsButton.vue"]) {
      expect(read(f), f).toMatch(/\.header-btn-subtitle \{\s*font-size: 0\.72rem;/);
    }
    expect(read("../pages/home.vue")).toContain('class="required-assignees d-flex align-center ga-1"');
  });
});
