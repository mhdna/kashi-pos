// @vitest-environment happy-dom
//
// The Balance page's month list: its original compact look, with a Today tag and
// a Day / Visitors / Total header over aligned columns.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { ref } from "vue";
import ui from "@/ui";

vi.mock("@/composables/useCurrencies", () => ({
  useCurrencies: () => ({ currencies: ref([]), roundingMode: ref("half_up") }),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({ ListCurrencies: vi.fn(), GetAppSettings: vi.fn() }));

import MonthlyIncomeCard from "./MonthlyIncomeCard.vue";

let wrapper: VueWrapper<any> | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
});

const day = (d: string, total: number, shifts: any[] = []) => ({ day: d, total, shifts });
const ROWS = [
  day("11/11/2026", 288300),
  day("12/11/2026", 0),
  day("13/11/2026", 0), // today
  day("14/11/2026", 0), // to come
  day("15/11/2026", 5000), // to come, with something on it
];

async function mountCard(props: Record<string, unknown> = {}) {
  wrapper = mount(MonthlyIncomeCard, {
    props: {
      month: "11/2026",
      rows: ROWS,
      grandTotal: 293300,
      couponTotal: 0,
      onAccountTotal: 0,
      loading: false,
      expandedDays: [],
      allDaysExpanded: false,
      shiftExpanded: new Set<number>(),
      balanceCache: {},
      balanceLoading: {},
      currentDay: "13/11/2026",
      isAdmin: false,
      canChangeMonth: false,
      printingShiftId: null,
      targets: [],
      visitorCounts: { "11/11/2026": 11, "12/11/2026": 0, "13/11/2026": 1135 },
      visitorCounterEnabled: true,
      atCurrentMonth: true,
      ...props,
    },
    global: { plugins: [ui] },
  });
  await flushPromises();
  return wrapper;
}
const row = (w: VueWrapper<any>, d: string) => w.find(`[data-day="${d}"]`);

describe("month list", () => {
  it("lists every day with its date and total, as before", async () => {
    const w = await mountCard();
    expect(w.findAll(".monthly-income-row")).toHaveLength(5);
    expect(row(w, "11/11/2026").text()).toContain("11/11/2026");
    expect(row(w, "11/11/2026").find(".mi-total").text()).toBe("$2,883.00");
    // A day with nothing sold and a day still to come both read $0.00: no dimming, no dash.
    expect(row(w, "12/11/2026").find(".mi-total").text()).toBe("$0.00");
    expect(row(w, "14/11/2026").find(".mi-total").text()).toBe("$0.00");
    expect(row(w, "15/11/2026").find(".mi-total").text()).toBe("$50.00");
  });

  it("picks out today with the grey row it always had, and a Today tag", async () => {
    const w = await mountCard();
    expect(row(w, "13/11/2026").classes()).toContain("monthly-income-current-day");
    expect(row(w, "12/11/2026").classes()).not.toContain("monthly-income-current-day");
    expect(w.findAll("[data-test=today-tag]")).toHaveLength(1);
    expect(row(w, "13/11/2026").find("[data-test=today-tag]").text()).toBe("Today");
    expect(row(w, "13/11/2026").find(".mi-date, .mi-day").text()).toContain("13/11/2026");
    expect(w.find(".mi-weekday").exists()).toBe(false);
  });

  it("has no Today tag when today is not in the month shown", async () => {
    const w = await mountCard({ currentDay: "05/12/2026" });
    expect(w.find("[data-test=today-tag]").exists()).toBe(false);
  });

  it("the tag follows today: another day gets it when it is another day", async () => {
    const w = await mountCard({ currentDay: "12/11/2026" });
    expect(row(w, "12/11/2026").find("[data-test=today-tag]").exists()).toBe(true);
    expect(row(w, "13/11/2026").find("[data-test=today-tag]").exists()).toBe(false);
  });

  it("the total row says the month's total", async () => {
    const w = await mountCard();
    expect(w.find("[data-test=income-grand-total]").text()).toContain("$2,933.00");
  });
});

describe("compact list", () => {
  const css = readFileSync(resolve(process.cwd(), "src/components/shifts/MonthlyIncomeCard.vue"), "utf8");

  it("has compact rows, header and total: 30px, as it always had", () => {
    expect(Number(css.match(/--mi-row: (\d+)px/)![1])).toBe(30);
    expect(css).toMatch(/\.monthly-income-row \{[^}]*min-height: var\(--mi-row\)/);
    expect(css).toMatch(/\.monthly-income-total \{[^}]*min-height: var\(--mi-row\)/);
    expect(css).toMatch(/\.monthly-income-header \{[^}]*min-height: 30px/);
    expect(css).not.toMatch(/min-height: (3[1-9]|4\d|5\d)px/);
  });

  it("has the small compact chevron and the extra-small visitors chip", async () => {
    const w = await mountCard();
    const chevron = row(w, "11/11/2026").find(".monthly-income-expand").classes().join(" ");
    expect(chevron).toContain("compact");
    expect(chevron).toContain("small");
    expect(row(w, "11/11/2026").find("[data-test=visitors]").classes().join(" ")).toContain("x-small");
  });

  it("lines the visitors up down the list: left-aligned in a column, header included, clear of the total", () => {
    expect(css).toMatch(/\.mi-visitors-cell \{[^}]*justify-content: flex-start/);
    expect(css).toMatch(/\.mi-visitors-head \{[^}]*text-align: start/);
    // The total has a column of its own (right-aligned), so the visitors sit left of it.
    expect(css).toMatch(/1fr 84px 104px/);
    expect(css).toMatch(/\.mi-total-head,\s*\.mi-total \{[^}]*text-align: end/);
  });

  it("the header sits over the rows' columns: the same first-column width and padding", () => {
    expect(css).toMatch(/\.monthly-income-header \{[^}]*grid-template-columns: var\(--mi-chevron\) 1fr auto/);
    expect(css).toMatch(/\.monthly-income-row\.monthly-income--visitors \{[^}]*grid-template-columns: var\(--mi-chevron\) 1fr 84px 104px/);
    expect(css).toMatch(/\.monthly-income-header\.monthly-income--visitors \{[^}]*grid-template-columns: var\(--mi-chevron\) 1fr 84px 104px/);
    expect(css).toMatch(/\.monthly-income-header \{[^}]*padding: 0 12px 0 0/);
    expect(css).toMatch(/\.monthly-income-row \{[^}]*padding: 0 12px 0 0/);
  });
});

describe("visitors column", () => {
  it("has its own column in the header and every row, in the same place", async () => {
    const w = await mountCard();
    expect(w.find("[data-test=income-header]").text()).toContain("Visitors");
    expect(w.find("[data-test=income-header]").classes()).toContain("monthly-income--visitors");
    for (const r of w.findAll(".monthly-income-row")) expect(r.classes()).toContain("monthly-income--visitors");
    // Four cells per row: chevron, day, visitors, total.
    expect(row(w, "11/11/2026").element.children.length).toBe(4);
  });

  it("shows the count, 0 included, and nothing for a day with no count", async () => {
    const w = await mountCard();
    expect(row(w, "11/11/2026").find("[data-test=visitors]").text()).toBe("11");
    expect(row(w, "13/11/2026").find("[data-test=visitors]").text()).toBe("1135");
    expect(row(w, "12/11/2026").find("[data-test=visitors]").text()).toBe("0");
    expect(row(w, "14/11/2026").find("[data-test=visitors]").exists()).toBe(false);
  });

  it("the tooltip says how many people, and what each spent", async () => {
    const w = await mountCard();
    expect(row(w, "11/11/2026").find("[data-test=visitors]").attributes("title")).toBe(
      "11 people visited · $262.09 per visitor",
    );
    // No takings, visitors: $0.00 each; and a single visitor is a person.
    expect(row(w, "12/11/2026").find("[data-test=visitors]").attributes("title")).toBe("0 people visited");
    const one = await mountCard({ rows: [day("11/11/2026", 4000)], visitorCounts: { "11/11/2026": 1 } });
    expect(one.find("[data-test=visitors]").attributes("title")).toBe("1 person visited · $40.00 per visitor");
  });

  it("is gone when the visitor counter is off, and the rows have three cells", async () => {
    const w = await mountCard({ visitorCounterEnabled: false });
    expect(w.find("[data-test=income-header]").text()).not.toContain("Visitors");
    expect(w.find("[data-test=visitors]").exists()).toBe(false);
    expect(row(w, "11/11/2026").element.children.length).toBe(3);
    expect(w.find("[data-test=income-header]").classes()).not.toContain("monthly-income--visitors");
  });
});

describe("opening a day", () => {
  it("a day with no shifts says so", async () => {
    const w = await mountCard({ expandedDays: ["12/11/2026"] });
    expect(w.find("[data-test=day-empty]").text()).toBe("No data found for this day.");
  });

  it("tapping a row asks to open it", async () => {
    const w = await mountCard();
    await row(w, "11/11/2026").trigger("click");
    expect(w.emitted("toggle-day")).toEqual([["11/11/2026"]]);
  });

  it("the chevron opens the same day, once", async () => {
    const w = await mountCard();
    await row(w, "12/11/2026").find(".monthly-income-expand").trigger("click");
    expect(w.emitted("toggle-day")).toEqual([["12/11/2026"]]);
  });
});
