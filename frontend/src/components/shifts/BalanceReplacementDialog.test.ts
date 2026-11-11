// @vitest-environment happy-dom
//
// The Swap balance dialog as the cashier uses it: accounts are tiles with their
// method and currency icons, "All" fills the whole balance, the backend's
// preview reads as a sentence, and Swap sends what the preview showed.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { ref } from "vue";
import ui from "@/ui";

const auth = { isAdmin: false };
vi.mock("@/stores/auth", () => ({ useAuthStore: () => auth }));
vi.mock("@/composables/useCurrencies", () => ({
  useCurrencies: () => ({
    currencies: ref([
      { code: "USD", symbol: "$", unitsPerUsd: 1, cashRoundingUnit: 1 },
      { code: "LBP", symbol: "LL", unitsPerUsd: 89000, cashRoundingUnit: 1000 },
    ]),
    roundingMode: ref("half_up"),
  }),
  findCurrency: (list: any[], code: string) => list.find((c) => c.code === code),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({
  ReplaceShiftBalance: vi.fn(),
  PreviewShiftBalanceReplacement: vi.fn(),
  ConvertCashEntryToCents: vi.fn(),
}));

import { ConvertCashEntryToCents, PreviewShiftBalanceReplacement, ReplaceShiftBalance } from "../../../wailsjs/go/main/App";
import BalanceReplacementDialog from "./BalanceReplacementDialog.vue";

const convert = vi.mocked(ConvertCashEntryToCents);
const previewCall = vi.mocked(PreviewShiftBalanceReplacement);
const replace = vi.mocked(ReplaceShiftBalance);

// Cash USD $250.00, Cash LL 1,780,000 LL (= $20.00), Visa $90.00.
const SOURCES = [
  { accountId: 1, accountName: "Cash $", balance: 25000, currencyCode: "USD" },
  { accountId: 2, accountName: "Cash LL", balance: 2000, currencyCode: "LBP" },
  { accountId: 3, accountName: "Visa", balance: 9000, currencyCode: "USD" },
];
const METHODS = [
  { id: 1, name: "Cash $", currencyCode: "USD" },
  { id: 2, name: "Cash LL", currencyCode: "LBP" },
  { id: 3, name: "Visa", currencyCode: "USD" },
];

let wrapper: VueWrapper<any> | null = null;
const q = <T extends Element = HTMLElement>(s: string) => document.body.querySelector<T>(s);
const qa = (s: string) => Array.from(document.body.querySelectorAll<HTMLElement>(s));
const settle = async () => {
  await flushPromises();
  await new Promise((r) => setTimeout(r, 200)); // the preview's debounce
  await flushPromises();
};

async function open(sources = SOURCES) {
  wrapper = mount(BalanceReplacementDialog, {
    props: { modelValue: false, shiftId: 7, sources, paymentMethods: METHODS },
    global: { plugins: [ui] },
    attachTo: document.body,
  });
  await wrapper.setProps({ modelValue: true });
  await settle();
}

const tiles = (kind: "source" | "destination") => qa(`[data-test="swap-${kind}"]`);
const selected = (kind: "source" | "destination") =>
  tiles(kind).find((t) => t.classList.contains("swap-tile--selected"))?.querySelector(".swap-tile__name")?.textContent?.trim();
const amountInput = () => q<HTMLInputElement>(".swap-dialog__amount input")!;
async function click(el: Element | null | undefined) {
  expect(el, "element to click").toBeTruthy();
  (el as HTMLElement).click();
  await settle();
}
async function type(value: string) {
  const input = amountInput();
  input.value = value;
  input.dispatchEvent(new Event("input"));
  await settle();
}

beforeEach(() => {
  vi.clearAllMocks();
  auth.isAdmin = false;
  convert.mockImplementation(async (code: string, amount: number) =>
    code === "LBP" ? Math.round((amount / 89000) * 100) : Math.round(amount * 100),
  );
  previewCall.mockImplementation(async (_shift: number, _src: number, dst: number, cents: number) => ({
    valid: true,
    reason: "",
    sourceRemainingCents: 0,
    destinationReceivesCents: dst === 2 ? cents : cents,
  }) as any);
  replace.mockResolvedValue(undefined as any);
});
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  document.body.innerHTML = "";
});

describe("Swap balance dialog", () => {
  it("shows every account as a tile with its balance, method icon and currency icon", async () => {
    await open();
    const sourceTiles = tiles("source");
    expect(sourceTiles.map((t) => t.querySelector(".swap-tile__name")?.textContent?.trim())).toEqual(["Cash $", "Cash LL", "Visa"]);
    expect(sourceTiles[0]!.querySelector(".swap-tile__balance")?.textContent).toContain("250");
    expect(sourceTiles.every((t) => t.querySelectorAll("svg").length >= 2)).toBe(true);
  });

  it("starts from the physical-cash account and fills its whole balance", async () => {
    await open();
    expect(selected("source")).toBe("Cash LL");
    expect(amountInput().value).toBe("1780000");
    // The source isn't offered as its own destination.
    expect(tiles("destination").map((t) => t.querySelector(".swap-tile__name")?.textContent?.trim())).toEqual(["Cash $", "Visa"]);
    expect(selected("destination")).toBe("Cash $");
  });

  it("picks accounts by tapping tiles", async () => {
    await open();
    await click(tiles("source")[2]);
    expect(selected("source")).toBe("Visa");
    expect(amountInput().value).toBe("90");
    await click(tiles("destination")[1]);
    expect(selected("destination")).toBe("Cash LL");
    expect(tiles("destination")[1]!.getAttribute("aria-pressed")).toBe("true");
  });

  it("'All' puts the whole balance back after the amount was changed", async () => {
    await open();
    await click(tiles("source")[0]);
    await type("12");
    expect(amountInput().value).toBe("12");
    await click(q('[data-test="swap-all"]'));
    expect(amountInput().value).toBe("250");
  });

  it("says what the destination receives, and swaps exactly that", async () => {
    await open();
    await click(tiles("source")[0]);
    await type("40");
    expect(q('[data-test="swap-summary"]')?.textContent).toMatch(/Cash LL\s*receives/);
    expect(previewCall).toHaveBeenLastCalledWith(7, 1, 2, 4000, false);
    await click(q('[data-test="swap-save"]'));
    expect(replace).toHaveBeenCalledWith(7, 1, 2, 4000, false);
    expect(wrapper!.emitted("saved")?.[0]).toEqual([7]);
  });

  it("shows the backend's reason and keeps Swap off when the preview is invalid", async () => {
    previewCall.mockResolvedValue({ valid: false, reason: "More than the balance", sourceRemainingCents: 0, destinationReceivesCents: 0 } as any);
    await open();
    expect(q('[data-test="swap-error"]')?.textContent).toContain("More than the balance");
    expect(q<HTMLButtonElement>('[data-test="swap-save"]')!.disabled).toBe(true);
    await click(q('[data-test="swap-save"]'));
    expect(replace).not.toHaveBeenCalled();
  });

  it("hides the Swap/Transfer switch from a cashier", async () => {
    await open();
    expect(q('[data-test="swap-mode"]')).toBeNull();
  });

  it("lets an admin switch to Transfer, which starts empty and sends transfer=true", async () => {
    auth.isAdmin = true;
    await open();
    const transferBtn = qa('[data-test="swap-mode"] button').find((b) => b.textContent?.includes("Transfer"));
    await click(transferBtn);
    expect(q(".swap-dialog .text-h6")?.textContent).toBe("Transfer balance");
    expect(amountInput().value).toBe("");
    expect(q('[data-test="swap-all"]')).toBeNull();
    await type("5000000");
    await click(q('[data-test="swap-save"]'));
    expect(replace).toHaveBeenCalledWith(7, 2, 1, 5618, true);
  });
});
