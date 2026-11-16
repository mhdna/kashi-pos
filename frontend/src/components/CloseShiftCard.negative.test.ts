// @vitest-environment happy-dom
//
// The close-shift screen with a shift whose LBP and Visa ended negative:
// autofill shows the minus sign, the cashier can close straight away, and
// what reaches the backend is the negative count. No balance is moved
// first (the old auto swap into Cash $$ is gone).
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

vi.mock("vue-router", () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock("@/stores/auth", () => ({ useAuthStore: () => ({ logout: vi.fn() }) }));
vi.mock("@/lib/sound", () => ({ playErrorSound: vi.fn(), playSuccessSound: vi.fn() }));
vi.mock("../../wailsjs/runtime/runtime", () => ({ EventsOn: vi.fn(() => () => {}) }));
vi.mock("../../wailsjs/go/main/App", () => ({
  GetOpenShift: vi.fn(),
  ListShiftBalances: vi.fn(),
  ListSalespersons: vi.fn(),
  CloseShiftByID: vi.fn(),
  OpenShift: vi.fn(),
  GetShiftExpectedTotals: vi.fn(),
  CheckShiftReceiptCaptured: vi.fn(),
  ShutDownComputer: vi.fn(),
  SkipShiftReceiptUpload: vi.fn(),
  SwitchShiftReceiptToLocal: vi.fn(),
  GetSpendSummary: vi.fn(),
  ListCurrencies: vi.fn().mockResolvedValue([]),
  GetAppSettings: vi.fn().mockResolvedValue({}),
  GetSettings: vi.fn().mockResolvedValue({}),
  GetCloseShiftSyncGate: vi.fn().mockResolvedValue({ required: false }),
  BypassCloseShiftSyncGate: vi.fn(),
  CancelSyncFlush: vi.fn(),
  FlushSyncOutbox: vi.fn(),
}));

import * as App from "../../wailsjs/go/main/App";
import CloseShiftCard from "./CloseShiftCard.vue";

const api = vi.mocked(App) as any;
let wrapper: VueWrapper<any> | null = null;

beforeEach(() => {
  api.GetOpenShift.mockResolvedValue({ id: 42, createdAt: "2026-11-16 08:00:00" });
  api.ListShiftBalances.mockResolvedValue([
    { accountId: 1, accountName: "Cash $$", balance: 10000, currencyCode: "USD" },
    { accountId: 2, accountName: "Cash LL", balance: -766, currencyCode: "LBP" },
  ]);
  api.ListSalespersons.mockResolvedValue([{ id: 7, fullName: "Rami", isCurrent: true }]);
  api.GetSpendSummary.mockResolvedValue({});
  api.GetShiftExpectedTotals.mockResolvedValue({
    expectedUsd: 10000, expectedLbp: -686000, expectedVisa: -1250, expectedWhish: 0,
    usdBillCounts: { "100": 1 },
  });
  api.CloseShiftByID.mockResolvedValue({ receiptPrinted: false });
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  document.body.innerHTML = "";
  vi.clearAllMocks();
});

async function show() {
  wrapper = mount(CloseShiftCard, { global: { plugins: [ui] }, attachTo: document.body });
  await flushPromises();
  // The till has one closing person to pick.
  (wrapper.vm as any).closingPersonId = 7;
  await flushPromises();
  return wrapper;
}

const field = (label: string) =>
  wrapper!.findAll(".k-field, .k-text-field").find((f) => f.text().includes(label));
const inputValue = (label: string) => (field(label)?.find("input").element as HTMLInputElement | undefined)?.value;
const closeButton = () => wrapper!.findAll("button").find((b) => b.text() === "Close Shift")!;

describe("close shift with negative counts", () => {
  it("autofill fills the negative LBP and Visa", async () => {
    await show();
    expect(inputValue("LBP")).toBe("-686000");
    expect(inputValue("Visa")).toBe("-12.50");
    expect(wrapper!.text()).toContain("-686,000 LBP");
  });

  it("the negative count can be submitted and reaches the backend as is", async () => {
    await show();
    expect(closeButton().attributes("disabled")).toBeUndefined();
    await closeButton().trigger("click");
    await flushPromises();
    expect(api.CloseShiftByID).toHaveBeenCalledTimes(1);
    const [shiftId, personId, lbp, visa, whish, bills] = api.CloseShiftByID.mock.calls[0];
    expect([shiftId, personId, lbp, visa, whish]).toEqual([42, 7, -686000, -1250, 0]);
    expect(JSON.parse(bills)["100"]).toBe(1);
  });

  it("the LBP pad has a +/− key that flips the count", async () => {
    await show();
    const activator = field("LBP")!.find("input").element;
    activator.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();
    const sign = document.body.querySelector("[data-test=number-pad-sign]") as HTMLElement;
    expect(sign).not.toBeNull();
    sign.click();
    await flushPromises();
    expect(inputValue("LBP")).toBe("686000");
    sign.click();
    await flushPromises();
    expect(inputValue("LBP")).toBe("-686000");
  });

  it("USD bill pads have no +/− key", async () => {
    await show();
    const activator = field("$20")!.find("input").element;
    activator.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();
    expect(document.body.querySelector("[data-test=number-pad-backspace]")).not.toBeNull(); // the pad is open
    expect(document.body.querySelector("[data-test=number-pad-sign]")).toBeNull();
  });
});
