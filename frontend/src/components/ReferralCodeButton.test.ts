// @vitest-environment happy-dom
//
// The sale screen's referral code button: hidden while referrals are off,
// catches a typo before asking the backend, shows the backend's refusal,
// and hands back the checked code. A new customer starts over.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { nextTick } from "vue";
import ui from "@/ui";

vi.mock("../../wailsjs/go/main/App", () => ({
  CheckReferralCode: vi.fn(),
  GetReferralInfo: vi.fn(),
}));

import { CheckReferralCode, GetReferralInfo } from "../../wailsjs/go/main/App";
import ReferralCodeButton from "./ReferralCodeButton.vue";

const checkCode = vi.mocked(CheckReferralCode);
const getInfo = vi.mocked(GetReferralInfo);

let wrapper: VueWrapper | null = null;

async function mountButton(clientId = 7, modelValue = "") {
  wrapper = mount(ReferralCodeButton, {
    props: { modelValue, clientId, "onUpdate:modelValue": (v: string) => wrapper?.setProps({ modelValue: v }) },
    global: { plugins: [ui] },
    attachTo: document.body,
  });
  await flushPromises();
  return wrapper;
}

function find(sel: string): HTMLElement | null {
  return document.body.querySelector(`[data-test="${sel}"]`);
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
  checkCode.mockReset();
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  document.body.innerHTML = "";
});

describe("ReferralCodeButton", () => {
  it("is hidden while referrals are off", async () => {
    getInfo.mockResolvedValue({ enabled: false, rewardPercent: 0, code: "", rewardOfferPercent: 0 } as never);
    await mountButton();
    expect(find("referral-open")).toBeNull();
  });

  it("explains the reward and catches a typo without asking the backend", async () => {
    await mountButton();
    find("referral-open")!.click();
    await flushPromises();
    expect(document.body.textContent).toContain("15% off their next visit");
    await type("K7M4QA");
    expect(find("referral-message")?.textContent).toMatch(/typo/);
    find("referral-add")!.click();
    await flushPromises();
    expect(checkCode).not.toHaveBeenCalled();
  });

  it("adds a code the backend accepts, normalized", async () => {
    checkCode.mockResolvedValue({ code: "K7M4QH", ownerName: "Rami" } as never);
    const w = await mountButton(7);
    find("referral-open")!.click();
    await flushPromises();
    await type("k7m-4qh");
    find("referral-add")!.click();
    await flushPromises();
    expect(checkCode).toHaveBeenCalledWith("K7M4QH", 7);
    expect(w.emitted("update:modelValue")?.at(-1)).toEqual(["K7M4QH"]);
  });

  it("shows the backend's refusal and keeps no code", async () => {
    checkCode.mockRejectedValue("A referral code is only for a new customer's first purchase");
    const w = await mountButton(7);
    find("referral-open")!.click();
    await flushPromises();
    await type("K7M4QH");
    find("referral-add")!.click();
    await flushPromises();
    expect(find("referral-message")?.textContent).toContain("only for a new customer");
    expect(w.emitted("update:modelValue")).toBeUndefined();
  });

  it("drops the code when another customer is picked", async () => {
    const w = await mountButton(7, "K7M4QH");
    await w.setProps({ clientId: 9 });
    expect(w.emitted("update:modelValue")?.at(-1)).toEqual([""]);
  });
});
