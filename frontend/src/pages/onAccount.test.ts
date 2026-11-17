// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import ui from "@/ui";

const list = {
  clients: [
    { id: 1, name: "Big Debtor", phone: "+96171111111", invoiceCount: 2, total: 350000, paid: 0, outstanding: 350000, lastSaleAt: "2026-11-10 10:00:00" },
    { id: 2, name: "Partly Paid", phone: "", invoiceCount: 1, total: 400000, paid: 150000, outstanding: 250000, lastSaleAt: "2026-11-01 09:00:00" },
  ],
};
const GetClientAccountDetails = vi.fn(async (id: number) => ({
  balance: 250000,
  entries: [
    { id: 1, invoiceCode: "SA/1", amount: 400000, paid: 150000, outstanding: 250000, createdAt: "2026-11-01 09:00:00" },
    { id: 2, invoiceCode: "SA/0", amount: 1000, paid: 1000, outstanding: 0, createdAt: "2026-10-01 09:00:00" },
  ],
  id,
}));
const ListOnAccountClients = vi.fn(async (_month: string) => list);
const months = { list: [] as string[] };
const ListOnAccountMonths = vi.fn(async () => months.list);
vi.mock("../../wailsjs/go/main/App", () => ({
  ListOnAccountMonths: () => ListOnAccountMonths(),
  ListOnAccountClients: (m: string) => ListOnAccountClients(m),
  GetClientAccountDetails: (id: number) => GetClientAccountDetails(id),
}));

import OnAccount from "./onAccount.vue";

let wrapper: VueWrapper<any> | null = null;
afterEach(() => { wrapper?.unmount(); wrapper = null; months.list = []; });

async function show() {
  wrapper = mount(OnAccount, { global: { plugins: [ui] }, attachTo: document.body });
  await flushPromises();
  return wrapper;
}

describe("On Account page", () => {
  it("says so when the till can't answer, instead of spinning forever", async () => {
    ListOnAccountClients.mockRejectedValueOnce(new Error("boom"));
    const w = await show();
    expect(w.find('[data-test="on-account-error"]').text()).toContain("boom");
  });
  it("the picker offers only months with debts, and opens on the newest when this month has none", async () => {
    months.list = ["2026-03", "2025-12"];
    const w = await show();
    expect(ListOnAccountClients).toHaveBeenLastCalledWith("2026-03");
    const choices = (w.vm as any).$?.setupState?.monthChoices ?? (w.vm as any).monthChoices;
    expect(choices.map((c: any) => c.value)).toEqual(["2026-03", "2025-12", ""]);
  });
  it("opens on the current month, with no title icon, and the owes chip is red text with no fill", async () => {
    const now0 = new Date();
    months.list = [`${now0.getFullYear()}-${String(now0.getMonth() + 1).padStart(2, "0")}`, "2020-01"];
    const w = await show();
    const now = new Date();
    expect(ListOnAccountClients).toHaveBeenLastCalledWith(`${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`);
    expect(w.find(".records-title .k-icon").exists()).toBe(false);
    const css = readFileSync(resolve(process.cwd(), "src/pages/onAccount.vue"), "utf8");
    expect(css).toMatch(/\.owes-chip \{[^}]*color: rgb\(var\(--k-theme-error\)\)/);
    expect(css).not.toMatch(/\.owes-chip \{[^}]*background/);
    expect(css).not.toContain("k-chip");
    expect(css).not.toMatch(/\.on-account-total \{[^}]*border/);
    expect(css).not.toContain("Payments are taken from");
  });
  it("lists every client with what they owe, and totals the three numbers", async () => {
    const w = await show();
    const text = w.text();
    for (const label of ["Sold", "Collected", "Owed"]) expect(w.find(".on-account-totals").text()).toContain(label);
    expect(text).not.toContain("Paid back");
    expect(text).toContain("Big Debtor");
    expect(text).toContain("Partly Paid");
    expect(text).toContain("$3,500.00");
    expect(w.find('[data-test="total-sold"]').text()).toBe("$7,500.00");
    expect(w.find('[data-test="total-paid"]').text()).toBe("$1,500.00");
    expect(w.find('[data-test="total-owed"]').text()).toBe("$6,000.00");
  });
  it("searching narrows the rows and the totals", async () => {
    const w = await show();
    await w.find('input').setValue("partly");
    expect(w.text()).not.toContain("Big Debtor");
    expect(w.find('[data-test="total-owed"]').text()).toBe("$2,500.00");
  });
  it("clicking a client shows only the invoices still owed", async () => {
    const w = await show();
    const row = w.findAll("tbody tr").find((r) => r.text().includes("Partly Paid"))!;
    await row.trigger("click");
    await flushPromises();
    expect(GetClientAccountDetails).toHaveBeenCalledWith(2);
    const entries = document.body.querySelectorAll('[data-test="detail-entry"]');
    expect(entries).toHaveLength(1);
    expect(entries[0]!.textContent).toContain("SA/1");
    expect(document.body.querySelector('[data-test="detail-balance"]')!.textContent).toContain("$2,500.00");
  });
  it("is a manager-only route; its balance card button is hidden for now (kept in a comment)", () => {
    const router = readFileSync(resolve(process.cwd(), "src/router/index.ts"), "utf8");
    expect(router).toMatch(/path: "on-account",\s*component: \(\) => import\("@\/pages\/onAccount.vue"\),\s*meta: \{ requiresManager: true \}/);
    const card = readFileSync(resolve(process.cwd(), "src/components/shifts/ShiftIncomeCard.vue"), "utf8");
    expect(card.replace(/<!--[\s\S]*?-->/g, "")).not.toContain("openManagerPage('/on-account')");
    expect(card).toContain("openManagerPage('/on-account')");
  });
});
