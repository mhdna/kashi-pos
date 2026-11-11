// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { ref } from "vue";
import ui from "@/ui";

const spend = { shiftExpenses: [] as any[], shiftLoans: [] as any[] };
vi.mock("../../../wailsjs/go/main/App", () => ({
  GetSpendSummary: vi.fn(async () => spend),
  VerifyManagerPassword: vi.fn(),
}));
vi.mock("vue-router", () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock("@/stores/auth", () => ({ useAuthStore: () => ({ isAdmin: true }) }));
vi.mock("@/composables/useManagerPass", () => ({ grantManagerPass: vi.fn() }));
vi.mock("@/composables/useCurrencies", () => ({
  useCurrencies: () => ({
    currencies: ref([
      { code: "USD", symbol: "$", unitsPerUsd: 1, cashRoundingUnit: 1 },
      { code: "LBP", symbol: "LL", unitsPerUsd: 8900000, cashRoundingUnit: 1000 },
    ]),
    roundingMode: ref("half_up"),
  }),
}));
vi.mock("@/components/shifts/BalanceReplacementDialog.vue", () => ({ default: { template: "<div />" } }));
vi.mock("@/components/shifts/SetOpeningFloatDialog.vue", () => ({ default: { template: "<div />" } }));

import ShiftIncomeCard from "./ShiftIncomeCard.vue";

let wrapper: VueWrapper<any> | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  spend.shiftExpenses = [];
  spend.shiftLoans = [];
});

async function show(items: any[]) {
  wrapper = mount(ShiftIncomeCard, {
    props: { items, title: "Current Shift Income", shiftId: 15, paymentMethods: [], balanceSources: [], loading: false },
    global: { plugins: [ui] },
  });
  await flushPromises();
  return wrapper;
}
const total = () => wrapper!.find('[data-test="income-total"]').text();

describe("ShiftIncomeCard", () => {
  it("each payment method gets its own icon, not the same wallet", async () => {
    await show([
      { accountName: "Cash $$", currencyCode: "USD", total: 243 },
      { accountName: "Cash LL", currencyCode: "LBP", total: 45900000 },
      { accountName: "Visa", currencyCode: "USD", total: 1998 },
      { accountName: "Whish", currencyCode: "USD", total: 132 },
    ]);
    const icons = wrapper!.findAll('[data-test="payment-icon"]');
    expect(icons).toHaveLength(4);
    const shapes = icons.map(i => i.html());
    expect(new Set(shapes).size).toBe(4);
  });
  it("the reported case: Cash $243.00 shows a total collected of $243.00", async () => {
    await show([{ accountId: 1, accountName: "Cash $$", currencyCode: "USD", total: 24300 }]);
    expect(wrapper!.text()).toContain("$243.00");
    expect(total()).toContain("Total collected");
    expect(total()).toContain("$243.00");
    expect(total()).not.toContain("$0.00");
  });

  it("adds every collected line and leaves sales on account out", async () => {
    await show([
      { accountId: 1, accountName: "Cash $$", currencyCode: "USD", total: 24300 },
      { accountId: 2, accountName: "Visa", currencyCode: "USD", total: 10050 },
      { accountId: 3, accountName: "On account", currencyCode: "USD", total: 9900, isOnAccount: true },
    ]);
    expect(total()).toContain("$343.50");
    expect(wrapper!.text()).toContain("On account");
    expect(wrapper!.text()).toContain("$99.00");
  });

  it("shows net income only when something went out, and subtracts it", async () => {
    await show([{ accountId: 1, accountName: "Cash $$", currencyCode: "USD", total: 24300 }]);
    expect(wrapper!.find('[data-test="net-income"]').exists()).toBe(false);
    wrapper!.unmount();

    spend.shiftExpenses = [{ id: 1, description: "Tape", amount: 500, currencyCode: "USD" }];
    await show([{ accountId: 1, accountName: "Cash $$", currencyCode: "USD", total: 24300 }]);
    expect(wrapper!.find('[data-test="net-income"]').text()).toContain("$238.00");
    expect(total()).toContain("$243.00");
  });

  it("says so when the shift has no income, with no total", async () => {
    await show([]);
    expect(wrapper!.find('[data-test="income-total"]').exists()).toBe(false);
    expect(wrapper!.text()).toContain("No income found for the current shift.");
  });

  // However many rows and expense lines there are, the buttons stay at the
  // bottom of the card: the lines scroll in their own box, the buttons sit
  // outside it.
  it("keeps the action buttons outside the scrolling body", async () => {
    const many = Array.from({ length: 30 }, (_, i) => ({ accountId: i + 1, accountName: `Account ${i + 1}`, currencyCode: "USD", total: 1000 }));
    await show(many);
    const body = wrapper!.find(".shift-income-body");
    const actions = wrapper!.find(".shift-income-actions");
    expect(body.exists()).toBe(true);
    expect(actions.exists()).toBe(true);
    expect(body.element.contains(actions.element)).toBe(false);
    expect(body.findAll(".k-list-item").length).toBe(30);
    expect(total()).toContain("$300.00");
    // Vitest doesn't load component CSS: the rules are read from the source.
    const css = readFileSync(resolve(process.cwd(), "src/components/shifts/ShiftIncomeCard.vue"), "utf8");
    expect(css).toMatch(/\.shift-income-body \{[^}]*overflow-y: auto/);
    expect(css).toMatch(/\.shift-income-body \{[^}]*min-height: 0/);
    // The body spans the card, so the total band (margin -16px) fills the width instead of being clipped.
    expect(css).toMatch(/\.shift-income-body \{[^}]*margin-inline: -16px/);
    expect(css).toMatch(/\.shift-income-body \{[^}]*padding-inline: 16px/);
    expect(css).toMatch(/\.shift-total \{[^}]*margin: 12px -16px/);
    expect(css).toMatch(/\.shift-income-actions \{[^}]*flex-shrink: 0/);
    // The rows run to the same edges as the total and the buttons (no extra side padding).
    expect(css).toMatch(/\.shift-income-card :deep\(\.k-list-item\) \{[^}]*padding-inline: 0/);
  });
});
