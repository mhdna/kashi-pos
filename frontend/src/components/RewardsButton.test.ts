// @vitest-environment happy-dom
//
// The sale screen's Rewards button: the deals the cashier picks (a student
// discount), the referral code, and the deals that apply by themselves for
// reference. Hidden when there's nothing; the number by it is what's on.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { nextTick } from "vue";
import ui from "@/ui";

vi.mock("../../wailsjs/go/main/App", () => ({
  CheckReferralCode: vi.fn(),
  GetReferralInfo: vi.fn(),
  ListRewardDeals: vi.fn(),
}));
const listeners = new Map<string, () => void>();
vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: vi.fn((name: string, cb: () => void) => {
    listeners.set(name, cb);
    return () => listeners.delete(name);
  }),
}));

import { CheckReferralCode, GetReferralInfo, ListRewardDeals } from "../../wailsjs/go/main/App";
import RewardsButton from "./RewardsButton.vue";

const checkCode = vi.mocked(CheckReferralCode);
const getInfo = vi.mocked(GetReferralInfo);
const listDeals = vi.mocked(ListRewardDeals);

const students = { uid: "students", name: "Students 15%", kind: "percent", percent: 15, amount: 0, minTotal: 0, cashierPicks: true };
const staff = { uid: "staff", name: "Staff $5", kind: "amount", percent: 0, amount: 500, minTotal: 0, cashierPicks: true };
const auto = { uid: "auto", name: "15% off $100", kind: "percent", percent: 15, amount: 0, minTotal: 10000, cashierPicks: false };

let wrapper: VueWrapper | null = null;

async function mountButton(props: Record<string, unknown> = {}) {
  wrapper = mount(RewardsButton, {
    props: {
      modelValue: "",
      picked: [],
      clientId: 7,
      appliedName: "",
      "onUpdate:modelValue": (v: string) => wrapper?.setProps({ modelValue: v }),
      "onUpdate:picked": (v: string[]) => wrapper?.setProps({ picked: v }),
      ...props,
    },
    global: { plugins: [ui] },
    attachTo: document.body,
  });
  await flushPromises();
  return wrapper;
}

function find(sel: string): HTMLElement | null {
  return document.body.querySelector(`[data-test="${sel}"]`);
}

async function openDialog() {
  find("rewards-open")!.click();
  await flushPromises();
}

async function type(text: string) {
  const input = find("referral-input")?.querySelector("input") ?? (find("referral-input") as HTMLInputElement | null);
  if (!input) throw new Error("no input");
  (input as HTMLInputElement).value = text;
  input.dispatchEvent(new Event("input"));
  await nextTick();
}

beforeEach(() => {
  getInfo.mockResolvedValue({ enabled: true, rewardPercent: 15, code: "", rewardOfferPercent: 0 } as never);
  listDeals.mockResolvedValue([students, staff, auto] as never);
  checkCode.mockReset();
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  document.body.innerHTML = "";
});

describe("RewardsButton", () => {
  it("the reported case: one Rewards button holding picked deals, the referral code and the automatic ones", async () => {
    await mountButton();
    expect(find("rewards-open")).not.toBeNull();
    expect(document.body.textContent).toContain("Rewards");
    await openDialog();
    expect(find("deal-students")?.textContent).toContain("Students 15%");
    expect(find("deal-students")?.textContent).toContain("15% off the whole invoice");
    expect(find("deal-staff")?.textContent).toContain("$5 off the whole invoice");
    expect(find("rewards-referral")).not.toBeNull();
    // Each deal is a card with its size up top; the automatic one is there
    // for reference: no switch, nothing to tap.
    expect(find("deal-students")?.textContent).toContain("15%");
    expect(find("deal-staff")?.textContent).toContain("$5");
    expect(find("auto-auto")?.textContent).toContain("15% off invoices of $100 or more");
    expect(find("auto-auto")?.textContent).toContain("Applies by itself");
    expect(find("auto-auto")?.tagName).not.toBe("BUTTON");
    expect(find("auto-auto")?.querySelector("input, button")).toBeNull();
    expect(find("rewards-automatic")?.textContent).toContain("Only kashi or an admin changes them");
    expect(document.body.textContent).not.toMatch(/[–—·•]/);
  });

  it("picking a deal turns it on for the sale, and the button shows it", async () => {
    const w = await mountButton();
    expect(find("rewards-badge")).toBeNull();
    await openDialog();
    expect(find("deal-students")?.getAttribute("aria-checked")).toBe("false");
    expect(find("deal-students")?.textContent).toContain("Tap to turn on");
    find("deal-students")!.click();
    await flushPromises();
    expect(w.emitted("update:picked")?.at(-1)).toEqual([["students"]]);
    expect(find("rewards-badge")?.textContent).toBe("1");
    expect(find("rewards-open")!.className).toContain("rewards-on");
    expect(find("deal-students")?.getAttribute("aria-checked")).toBe("true");
    expect(find("deal-students")?.className).toContain("reward-card--on");
    expect(find("deal-students")?.textContent).toContain("On for this sale");
    find("deal-students")!.click();
    await flushPromises();
    expect(w.emitted("update:picked")?.at(-1)).toEqual([[]]);
  });

  it("counts the code, each pick and a deal that applied by itself, the winner once", async () => {
    await mountButton({ modelValue: "K7M4QH", picked: ["students"], appliedName: "Students 15%" });
    expect(find("rewards-badge")?.textContent).toBe("2");
    wrapper!.unmount();
    await mountButton({ appliedName: "15% off $100" });
    expect(find("rewards-badge")?.textContent).toBe("1");
    await openDialog();
    expect(find("auto-applied")?.textContent).toBe("On this sale");
    expect(find("auto-auto")?.className).toContain("reward-card--applied");
    expect(find("rewards-now")?.textContent).toContain("15% off $100");
  });

  it("is hidden with no referrals and no deals; comes when central sends some", async () => {
    getInfo.mockResolvedValue({ enabled: false, rewardPercent: 0, code: "", rewardOfferPercent: 0 } as never);
    listDeals.mockResolvedValue([] as never);
    await mountButton();
    expect(find("rewards-open")).toBeNull();
    listDeals.mockResolvedValue([students] as never);
    listeners.get("sync_applied:invoice_discounts")?.();
    await flushPromises();
    expect(find("rewards-open")).not.toBeNull();
    getInfo.mockResolvedValue({ enabled: true, rewardPercent: 15, code: "", rewardOfferPercent: 0 } as never);
    listeners.get("sync_applied:referral_program")?.();
    await flushPromises();
    await openDialog();
    expect(find("rewards-referral")).not.toBeNull();
  });

  it("drops a picked deal central took away", async () => {
    listDeals.mockResolvedValue([staff] as never);
    const w = await mountButton({ picked: ["students", "staff"] });
    expect(w.emitted("update:picked")?.at(-1)).toEqual([["staff"]]);
  });

  it("stops listening when the screen goes", async () => {
    await mountButton();
    expect(listeners.has("sync_applied:referral_program")).toBe(true);
    expect(listeners.has("sync_applied:invoice_discounts")).toBe(true);
    wrapper?.unmount();
    wrapper = null;
    expect(listeners.size).toBe(0);
  });

  it("explains the reward and catches a typo without asking the backend", async () => {
    await mountButton();
    await openDialog();
    expect(document.body.textContent).toContain("15% off their next visit");
    await type("K7M4QA");
    expect(find("referral-message")?.textContent).toMatch(/typo/);
    find("referral-add")!.click();
    await flushPromises();
    expect(checkCode).not.toHaveBeenCalled();
  });

  it("adds a code the backend accepts, normalized, and can remove it", async () => {
    checkCode.mockResolvedValue({ code: "K7M4QH", ownerName: "Rami" } as never);
    const w = await mountButton();
    await openDialog();
    await type("k7m-4qh");
    find("referral-add")!.click();
    await flushPromises();
    expect(checkCode).toHaveBeenCalledWith("K7M4QH", 7);
    expect(w.emitted("update:modelValue")?.at(-1)).toEqual(["K7M4QH"]);
    expect(find("referral-current")?.textContent).toContain("Sent by Rami");
    find("referral-remove")!.click();
    await flushPromises();
    expect(w.emitted("update:modelValue")?.at(-1)).toEqual([""]);
  });

  it("shows the backend's refusal and keeps no code", async () => {
    checkCode.mockRejectedValue("A referral code is only for a new customer's first purchase");
    const w = await mountButton();
    await openDialog();
    await type("K7M4QH");
    find("referral-add")!.click();
    await flushPromises();
    expect(find("referral-message")?.textContent).toContain("only for a new customer");
    expect(w.emitted("update:modelValue")).toBeUndefined();
  });

  it("drops the code when another customer is picked", async () => {
    const w = await mountButton({ modelValue: "K7M4QH" });
    await w.setProps({ clientId: 9 });
    expect(w.emitted("update:modelValue")?.at(-1)).toEqual([""]);
  });
});

describe("RewardsButton cards, the reported set", () => {
  it("shows the student discount to pick and the $50, $100 and $150 tiers in order", async () => {
    listDeals.mockResolvedValue([
      { uid: "s", name: "Students 15%", kind: "percent", percent: 15, amount: 0, minTotal: 0, cashierPicks: true },
      { uid: "t150", name: "20% above $150", kind: "percent", percent: 20, amount: 0, minTotal: 15000, cashierPicks: false },
      { uid: "t50", name: "10% above $50", kind: "percent", percent: 10, amount: 0, minTotal: 5000, cashierPicks: false },
      { uid: "t100", name: "15% above $100", kind: "percent", percent: 15, amount: 0, minTotal: 10000, cashierPicks: false },
    ] as never);
    await mountButton({ appliedName: "15% above $100" });
    await openDialog();
    const cards = [...document.body.querySelectorAll(".reward-card")].map((c) => c.getAttribute("data-test"));
    expect(cards).toEqual(["deal-s", "auto-t50", "auto-t100", "auto-t150", "rewards-referral"]);
    expect(find("auto-t100")?.className).toContain("reward-card--applied");
    expect(find("auto-t50")?.className).not.toContain("reward-card--applied");
    expect(find("auto-t150")?.textContent).toContain("20% off invoices of $150 or more");
  });
});
