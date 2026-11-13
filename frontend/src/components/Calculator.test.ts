// @vitest-environment happy-dom
//
// The calculator with the real UI kit and the Wails backend mocked: a sale
// paid in dollars and LL with LL change back, in sale, exchange and
// settlement mode. What it shows as paid, and the LL it sends, must be what
// checkout records (tx_invoice.go, lib/cashPayments.ts).
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

vi.mock("../../wailsjs/go/main/App", () => ({
  ListCurrencies: vi.fn(),
  ListCouponTypes: vi.fn(),
  ListPaymentMethods: vi.fn(),
  QuotePaymentStatus: vi.fn(),
  QuoteSettlementPaymentStatus: vi.fn(),
  QuoteExchangePaymentStatus: vi.fn(),
}));
vi.mock("../../wailsjs/runtime/runtime", () => ({ EventsOn: vi.fn(() => () => {}) }));
vi.mock("@/lib/display", async () => {
  const { ref } = await import("vue");
  const calculatorTotalCurrency = ref("USD");
  return {
    calculatorTotalCurrency,
    setCalculatorTotalCurrency: (code: string) => (calculatorTotalCurrency.value = code),
  };
});

import {
  ListCouponTypes,
  ListCurrencies,
  ListPaymentMethods,
  QuoteExchangePaymentStatus,
  QuotePaymentStatus,
  QuoteSettlementPaymentStatus,
} from "../../wailsjs/go/main/App";
import Calculator from "./Calculator.vue";

const READY = { diffCents: 0, isPaidInFull: true, isCashRounded: false, checkoutReady: true };

let wrapper: VueWrapper<any> | null = null;

beforeEach(() => {
  vi.mocked(ListPaymentMethods).mockResolvedValue([
    { id: 1, name: "Cash $$", currencyCode: "USD" },
    { id: 2, name: "Cash LL", currencyCode: "LBP" },
  ] as any);
  vi.mocked(ListCurrencies).mockResolvedValue([
    { code: "USD", symbol: "$", unitsPerUsd: 1, cashRoundingUnit: 1, isActive: true, isDefault: false },
    { code: "LBP", symbol: "LL", unitsPerUsd: 90000, cashRoundingUnit: 10000, isActive: true, isDefault: false },
  ] as any);
  vi.mocked(ListCouponTypes).mockResolvedValue([] as any);
  vi.mocked(QuotePaymentStatus).mockResolvedValue(READY as any);
  vi.mocked(QuoteExchangePaymentStatus).mockResolvedValue(READY as any);
  vi.mocked(QuoteSettlementPaymentStatus).mockResolvedValue(READY as any);
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  vi.clearAllMocks();
  document.body.innerHTML = "";
});

const MODES = {
  sale: { itemsJson: '[{"code":"SKU-4160","qty":1}]', hasItems: true },
  exchange: { itemsJson: '[{"code":"A","qty":-1},{"code":"SKU-4160","qty":1}]', hasItems: true, exchangeMode: true, exchangeOriginalInvoiceId: 7 },
  settlement: { settlementInvoiceId: 9, hasItems: true },
} as const;

async function mountCalculator(mode: keyof typeof MODES) {
  wrapper = mount(Calculator, {
    props: { totalCents: 4156, roundingMode: "half_up", roundingCurrency: "LBP", overchangeConfirmationEnabled: true, ...MODES[mode] },
    global: { plugins: [ui] },
    attachTo: document.body,
  });
  await flushPromises();
  return wrapper;
}

// The cashier types an amount and taps a payment method ("" taps it with
// nothing typed, which fills in what's left).
async function pay(w: VueWrapper<any>, method: string, typed: string) {
  w.vm.restoreHoldState({ ...w.vm.holdState(), currentInput: typed, activeMethod: method });
  w.vm.selectMethodAndConfirm(method);
  await flushPromises();
}

// SA/CT/2026/03167 at Centro: $41.56, paid $40 and LL 200,000, LL 60,000 back.
describe.each(Object.keys(MODES) as (keyof typeof MODES)[])("Calculator in %s mode", (mode) => {
  it("LL 60,000 back on $40 + LL 200,000 is exactly paid, with no 'too much change' prompt", async () => {
    const w = await mountCalculator(mode);
    await pay(w, "Cash $$", "40");
    await pay(w, "Cash LL", "200000");
    expect(w.vm.diff).toBeCloseTo(0.66, 5);

    await pay(w, "Cash LL", "");
    expect(document.body.textContent).not.toContain("Confirm change amount");
    expect(w.vm.diff).toBe(0);
    expect(w.text()).toContain("Paid");

    const sent = JSON.parse(w.vm.paymentsJSON);
    expect(sent).toEqual([
      { accountId: 1, amountCents: 4000, entryType: "payment" },
      { accountId: 2, amountCents: 222, entryType: "payment", amountUnits: 200000 },
      { accountId: 2, amountCents: -67, entryType: "change", amountUnits: 60000 },
    ]);
  });

  it("asks the backend with the LL it will record", async () => {
    const w = await mountCalculator(mode);
    await pay(w, "Cash LL", "200000");
    const quote =
      mode === "sale" ? QuotePaymentStatus : mode === "exchange" ? QuoteExchangePaymentStatus : QuoteSettlementPaymentStatus;
    const calls = vi.mocked(quote).mock.calls;
    const lastPayments = JSON.parse(String(calls[calls.length - 1]!.find((arg) => typeof arg === "string" && arg.includes("accountId"))));
    expect(lastPayments.at(-1)).toMatchObject({ accountId: 2, amountUnits: 200000 });
  });

  it("still asks before giving more LL back than is owed", async () => {
    const w = await mountCalculator(mode);
    await pay(w, "Cash $$", "40");
    await pay(w, "Cash LL", "200000");
    await pay(w, "Cash LL", "70000");
    expect(document.body.textContent).toContain("Confirm change amount");
  });
});
