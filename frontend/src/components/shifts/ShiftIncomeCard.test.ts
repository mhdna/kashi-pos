// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
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
  it("the reported case: Cash $243.00 shows an income total of $243.00", async () => {
    await show([{ accountId: 1, accountName: "Cash $$", currencyCode: "USD", total: 24300 }]);
    expect(wrapper!.text()).toContain("$243.00");
    expect(total()).toContain("Income total");
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
});
