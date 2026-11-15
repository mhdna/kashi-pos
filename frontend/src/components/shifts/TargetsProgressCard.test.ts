// @vitest-environment happy-dom
//
// The Targets block under the Balance month: one plain bar per target, low to
// high, with its dates and how far along it is, in words rather than symbols.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { mount } from "@vue/test-utils";
import ui from "@/ui";
import TargetsProgressCard, { type BranchTarget } from "./TargetsProgressCard.vue";

const target = (over: Partial<BranchTarget> = {}): BranchTarget => ({
  id: 1, dateFrom: "2026-11-01", dateTo: "2026-11-30", targetAmountCents: 4_000_000, achievedCents: 1_000_000, color: "", ...over,
});
const mountCard = (targets: BranchTarget[]) => mount(TargetsProgressCard, { props: { targets }, global: { plugins: [ui] } });

describe("TargetsProgressCard", () => {
  it("shows nothing when there are no targets", () => {
    expect(mountCard([]).find(".targets-progress").exists()).toBe(false);
  });

  it("has a plain Targets title, with no icon", () => {
    const w = mountCard([target()]);
    expect(w.find(".targets-progress-title").text()).toBe("Targets");
    expect(w.find(".targets-progress-title .k-icon").exists()).toBe(false);
  });

  it("says the dates and amounts with to and of, never a dash or a slash", () => {
    const w = mountCard([target()]);
    const dates = w.find("[data-test=target-dates]").text();
    const amounts = w.find("[data-test=target-amounts]").text();
    expect(dates).toMatch(/ to /);
    expect(amounts).toMatch(/\$10,000\.00 of \$40,000\.00\s*\(25%\)/);
    expect(dates + amounts).not.toMatch(/[–—]| \/ /);
  });

  it("fills the bar to the progress, capped at 100%, and lists low targets first", () => {
    const w = mountCard([target({ id: 2, targetAmountCents: 100, achievedCents: 500 }), target({ id: 1 })]);
    const fills = w.findAll(".target-fill").map((f) => f.attributes("style"));
    expect(fills[0]).toContain("width: 100%");
    expect(fills[1]).toContain("width: 25%");
  });

  it("draws the fill plain: no stripes and no pill corners", () => {
    const css = readFileSync(resolve(__dirname, "TargetsProgressCard.vue"), "utf8");
    const fill = css.slice(css.indexOf(".target-fill {"), css.indexOf("}", css.indexOf(".target-fill {")));
    expect(fill).not.toMatch(/repeating-linear-gradient|background-image/);
    expect(css).not.toMatch(/border-radius:\s*6px/);
  });
});
