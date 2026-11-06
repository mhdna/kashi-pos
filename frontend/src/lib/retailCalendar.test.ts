import { describe, expect, it } from "vitest";
import { eventsInRange, expandRule } from "./retailCalendar";

const iso = (d: Date) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
const firstDay = (rule: Parameters<typeof expandRule>[0], year: number) =>
  iso(expandRule(rule, year)[0]!.start);

describe("expandRule", () => {
  it("fixed, span, nth weekday", () => {
    expect(firstDay({ type: "fixed", month: 11, day: 22 }, 2026)).toBe("2026-11-22");
    // Black Friday: the day after the 4th Thursday of November.
    expect(firstDay({ type: "nthWeekday", month: 11, weekday: 4, n: 4, offset: 1 }, 2026)).toBe(
      "2026-11-27",
    );
    const [winter] = expandRule(
      { type: "span", from: [12, 26], to: [1, 10], toNextYear: true },
      2026,
    );
    expect([iso(winter!.start), iso(winter!.end)]).toEqual(["2026-12-26", "2027-01-10"]);
  });

  it("Easter, both churches, and Good Friday", () => {
    expect(firstDay({ type: "easter", kind: "western" }, 2026)).toBe("2026-04-05");
    expect(firstDay({ type: "easter", kind: "orthodox" }, 2026)).toBe("2026-04-12");
    expect(firstDay({ type: "easter", kind: "western", offset: -2 }, 2026)).toBe("2026-04-03");
  });

  it("Hijri dates from an anchor, a span, and twice in one year", () => {
    const fitr = expandRule({ type: "hijri", anchor: "2024-04-10", days: 3 }, 2026)[0]!;
    expect([iso(fitr.start), iso(fitr.end)]).toEqual(["2026-03-20", "2026-03-22"]);
    expect(
      expandRule({ type: "hijri", anchor: "2024-01-25" }, 2026).map((r) => iso(r.start)),
    ).toEqual(["2026-01-03", "2026-12-23"]);
  });
});

describe("eventsInRange", () => {
  const from = new Date(2026, 10, 1);
  const to = new Date(2026, 11, 31);

  it("uses the bundled catalog until kashi sends one", () => {
    const names = eventsInRange(from, to, []).map((e) => e.name);
    expect(names).toContain("Black Friday");
    expect(names).not.toContain("Cyber Monday"); // clothing is the default business
  });

  it("follows the catalog kashi sent, the business type and the hidden list", () => {
    const catalog = JSON.stringify({
      lists: { a: { title: "A", icon: "mdi-flag" } },
      businessTypes: { shop: { title: "Shop", icon: "mdi-store", lists: ["a"] } },
      events: [
        {
          list: "a",
          name: "Open day",
          color: "pink",
          icon: "mdi-flag",
          rule: { type: "fixed", month: 11, day: 5 },
        },
        {
          list: "a",
          name: "Hidden day",
          color: "pink",
          icon: "mdi-flag",
          rule: { type: "fixed", month: 11, day: 6 },
        },
      ],
    });
    const out = eventsInRange(from, to, [], {
      businessType: "shop",
      hiddenEvents: ["Hidden day"],
      catalog,
    });
    expect(out.map((e) => e.name)).toEqual(["Open day"]);
  });

  it("falls back to the bundled catalog when the sent one is broken", () => {
    const out = eventsInRange(from, to, [], {
      businessType: "clothing",
      hiddenEvents: [],
      catalog: "{oops",
    });
    expect(out.map((e) => e.name)).toContain("Black Friday");
  });
});
