import { describe, expect, it } from "vitest";
import { currentMonth, defaultMonth, filterOnAccountClients, monthChoices, monthTitle, onAccountTotals, type OnAccountClient } from "./onAccount";

const mk = (id: number, name: string, total: number, paid: number, phone = ""): OnAccountClient => ({
  id, name, phone, invoiceCount: 1, total, paid, outstanding: total - paid,
});

describe("onAccountTotals", () => {
  it("adds sold, paid and owed over every row", () => {
    const t = onAccountTotals([mk(1, "A", 1000, 0), mk(2, "B", 4000, 1500)]);
    expect(t).toEqual({ total: 5000, paid: 1500, outstanding: 3500 });
  });
  it("is zero with no rows", () => {
    expect(onAccountTotals([])).toEqual({ total: 0, paid: 0, outstanding: 0 });
  });
  it("randomized: owed always equals sold minus paid, one cent off is caught", () => {
    let seed = 7;
    const rnd = () => ((seed = (seed * 1103515245 + 12345) & 0x7fffffff) / 0x7fffffff);
    for (let run = 0; run < 300; run++) {
      const rows = Array.from({ length: 1 + Math.floor(rnd() * 12) }, (_, i) => {
        const total = 1 + Math.floor(rnd() * 500000);
        return mk(i + 1, `C${i}`, total, Math.floor(rnd() * total));
      });
      const t = onAccountTotals(rows);
      expect(t.outstanding).toBe(t.total - t.paid);
      expect(t.outstanding).toBe(rows.reduce((s, r) => s + r.outstanding, 0));
      expect(t.outstanding + 1).not.toBe(t.total - t.paid);
    }
  });
});

describe("filterOnAccountClients", () => {
  const rows = [mk(1, "Mahdi Store", 100, 0, "+961 71 111 222"), mk(2, "Zeina", 100, 0, "03 555 666")];
  it("empty search keeps everything", () => {
    expect(filterOnAccountClients(rows, "  ")).toHaveLength(2);
  });
  it("matches the name without caring about case", () => {
    expect(filterOnAccountClients(rows, "mahdi").map((r) => r.id)).toEqual([1]);
  });
  it("matches the phone ignoring spaces", () => {
    expect(filterOnAccountClients(rows, "03555").map((r) => r.id)).toEqual([2]);
  });
  it("nothing matches gives an empty list", () => {
    expect(filterOnAccountClients(rows, "nobody")).toEqual([]);
  });
});

describe("month picker", () => {
  it("current month pads single digits", () => {
    expect(currentMonth(new Date(2026, 10, 16))).toBe("2026-11");
    expect(currentMonth(new Date(2026, 0, 31))).toBe("2026-01");
  });
  it("lists only the months given, newest first, once each, then All months", () => {
    expect(monthChoices(["2025-12", "2026-11", "2026-02", "2026-11"])).toEqual([
      { title: "November 2026", value: "2026-11" },
      { title: "February 2026", value: "2026-02" },
      { title: "December 2025", value: "2025-12" },
      { title: "All months", value: "" },
    ]);
  });
  it("with no debts only All months is offered", () => {
    expect(monthChoices([])).toEqual([{ title: "All months", value: "" }]);
  });
  it("opens on this month when it has debts", () => {
    expect(defaultMonth(["2026-09", "2026-11"], new Date(2026, 10, 16))).toBe("2026-11");
  });
  it("else on the newest month with debts", () => {
    expect(defaultMonth(["2026-07", "2026-09"], new Date(2026, 10, 16))).toBe("2026-09");
  });
  it("else on all months", () => {
    expect(defaultMonth([], new Date(2026, 10, 16))).toBe("");
  });
  it("names every month", () => {
    for (let m = 1; m <= 12; m++) expect(monthTitle(`2026-${String(m).padStart(2, "0")}`)).not.toContain("undefined");
    expect(monthTitle("2026-01")).toBe("January 2026");
    expect(monthTitle("2026-12")).toBe("December 2026");
  });
});
