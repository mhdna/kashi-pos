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

// The code has its own button beside Rewards.
async function openCode() {
  find("referral-open")!.click();
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
    // Short cards: the size, then one line; the details are in the title.
    expect(find("deal-students")?.textContent).toContain("15%");
    // The name big on top (its size left out of it), the size under it.
    expect(find("deal-students")?.querySelector(".reward-line")?.textContent).toBe("Students");
    expect(find("deal-students")?.querySelector(".reward-size")?.textContent).toBe("15%");
    expect(find("deal-students")?.getAttribute("title")).toBe("15% off the whole invoice");
    expect(find("deal-staff")?.textContent).toContain("$5");
    expect(find("auto-auto")?.querySelector(".reward-size")?.textContent).toBe("15%");
    expect(find("auto-auto")?.querySelector(".reward-line")?.textContent).toBe("> $100");
    expect(find("auto-auto")?.tagName).not.toBe("BUTTON");
    expect(find("auto-auto")?.className).toContain("reward-card--locked");
    expect(find("auto-auto")?.querySelector("input, button")).toBeNull();
    // The reported case: no referral card here; the Referral button beside
    // Rewards holds the code.
    expect(find("rewards-referral")).toBeNull();
    expect(find("referral-input")).toBeNull();
    expect(find("rewards-automatic")).toBeNull();
    expect(document.body.textContent).toContain("Choose");
    expect(document.body.textContent).not.toContain("For this sale");
    expect(document.body.textContent).not.toContain("Locked");
    expect(document.body.textContent).not.toMatch(/[–—·•]/);
  });

  it("the reported case: one deal at a time, a new pick takes over the one before", async () => {
    const w = await mountButton();
    await openDialog();
    find("deal-students")!.click();
    await flushPromises();
    find("deal-staff")!.click();
    await flushPromises();
    expect(w.emitted("update:picked")?.at(-1)).toEqual([["staff"]]);
    expect(find("deal-staff")?.getAttribute("aria-checked")).toBe("true");
    expect(find("deal-students")?.getAttribute("aria-checked")).toBe("false");
    expect(document.querySelectorAll(".reward-card--on").length).toBe(1);
    expect(find("rewards-badge")?.textContent).toBe("1");
  });

  it("where the branch lets deals add up, several can be picked, and each applied one shows", async () => {
    listDeals.mockResolvedValue([students, staff, auto]);
    const w = await mountButton({ stack: true, appliedName: "15% off $100 + Students 15%" });
    await openDialog();
    find("deal-students")!.click();
    await flushPromises();
    find("deal-staff")!.click();
    await flushPromises();
    expect(w.emitted("update:picked")?.at(-1)).toEqual([["students", "staff"]]);
    expect(document.querySelectorAll(".reward-card--on").length).toBe(2);
    expect(find("auto-auto")?.className).toContain("reward-card--applied");
    // Two picked, one more applied by itself.
    expect(find("rewards-badge")?.textContent).toBe("3");
  });

  it("the reported case: a Referral button beside Rewards opens the code straight away", async () => {
    getInfo.mockResolvedValue({ enabled: true, rewardPercent: 15, code: "", rewardOfferPercent: 0 } as never);
    const w = await mountButton();
    expect(find("referral-open")).not.toBeNull();
    find("referral-open")!.click();
    await flushPromises();
    expect(find("referral-input")).not.toBeNull();
    // The Rewards dialog didn't open for it.
    expect(document.body.textContent).not.toContain("Choose");
    await w.setProps({ modelValue: "K7M4QH" });
    expect(find("referral-open-tick")).not.toBeNull();
  });

  it("picking a deal turns it on for the sale, and the button shows it", async () => {
    const w = await mountButton();
    expect(find("rewards-badge")).toBeNull();
    await openDialog();
    expect(find("deal-students")?.getAttribute("aria-checked")).toBe("false");
    find("deal-students")!.click();
    await flushPromises();
    expect(w.emitted("update:picked")?.at(-1)).toEqual([["students"]]);
    expect(find("rewards-badge")?.textContent).toBe("1");
    expect(find("rewards-open")!.className).toContain("rewards-on");
    expect(find("deal-students")?.getAttribute("aria-checked")).toBe("true");
    expect(find("deal-students")?.className).toContain("reward-card--on");
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
    expect(find("auto-applied")).not.toBeNull();
    expect(find("auto-auto")?.className).toContain("reward-card--applied");
    expect(find("rewards-now")).toBeNull();
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
    expect(find("referral-open")).not.toBeNull();
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

  it("the reported case: the field stops at a code's length, with no message while typing", async () => {
    await mountButton();
    await openCode();
    await type("k7m");
    expect(find("referral-message")).toBeNull();
    expect((find("referral-add") as HTMLButtonElement).disabled).toBe(true);
    await type("k7m-4qhxyz");
    await flushPromises();
    const input = (find("referral-input")?.querySelector("input") ?? find("referral-input")) as HTMLInputElement;
    expect(input.value).toBe("K7M4QH");
    expect((find("referral-add") as HTMLButtonElement).disabled).toBe(false);
    // The keyboard has no dash or space: a code never has them.
    const keys = [...document.body.querySelectorAll(".inline-keyboard button")].map((b) => b.textContent?.trim());
    expect(keys).not.toContain("-");
    expect(keys).not.toContain("Space");
  });

  it("explains the reward and catches a typo without asking the backend", async () => {
    await mountButton();
    await openCode();
    expect(document.body.textContent).not.toContain("Client referral code");
    // The keyboard has the number row and types capitals.
    const keys = [...document.body.querySelectorAll(".inline-keyboard-key")].map((k) => k.textContent?.trim());
    expect(keys.slice(0, 10)).toEqual(["1", "2", "3", "4", "5", "6", "7", "8", "9", "0"]);
    expect(keys).toContain("K");
    expect(keys).not.toContain("k");
    await type("K7M4QA");
    expect(find("referral-message")?.textContent).toMatch(/typo/);
    find("referral-add")!.click();
    await flushPromises();
    expect(checkCode).not.toHaveBeenCalled();
  });

  it("adds a code the backend accepts, normalized, and can remove it", async () => {
    checkCode.mockResolvedValue({ code: "K7M4QH", ownerName: "Rami" } as never);
    const w = await mountButton();
    await openCode();
    await type("k7m-4qh");
    find("referral-add")!.click();
    await flushPromises();
    expect(checkCode).toHaveBeenCalledWith("K7M4QH", 7);
    expect(w.emitted("update:modelValue")?.at(-1)).toEqual(["K7M4QH"]);
    // The Referral button shows it's on; its dialog closed.
    expect(find("referral-input")).toBeNull();
    expect(find("referral-open")?.className).toContain("rewards-on");
    expect(find("referral-open-tick")).not.toBeNull();
    find("referral-open")!.click();
    await flushPromises();
    expect(find("referral-current")?.textContent).toContain("Sent by Rami");
    find("referral-remove")!.click();
    await flushPromises();
    expect(w.emitted("update:modelValue")?.at(-1)).toEqual([""]);
  });

  it("shows the backend's refusal and keeps no code", async () => {
    checkCode.mockRejectedValue("A referral code is only for a new customer's first purchase");
    const w = await mountButton();
    await openCode();
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
    // Two rows: what the cashier picks, then the locked ones on their own.
    const row = (sel: string) => [...find(sel)!.querySelectorAll(".reward-card")].map((c) => c.getAttribute("data-test"));
    expect(row("rewards-pick-grid")).toEqual(["deal-s"]);
    expect(row("rewards-automatic-grid")).toEqual(["auto-t50", "auto-t100", "auto-t150"]);
    expect(find("auto-t100")?.className).toContain("reward-card--applied");
    expect(find("auto-t50")?.className).not.toContain("reward-card--applied");
    expect(find("auto-t150")?.querySelector(".reward-size")?.textContent).toBe("20%");
    expect(find("auto-t150")?.querySelector(".reward-line")?.textContent).toBe("> $150");
    expect(find("auto-t150")?.getAttribute("title")).toBe("20% off invoices of $150 or more");
  });
});
