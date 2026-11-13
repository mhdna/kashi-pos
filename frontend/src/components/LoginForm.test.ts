// @vitest-environment happy-dom
//
// The PIN keypad: C is a solid red key and backspace a soft red one; both stay
// greyed until a digit has been entered.
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

vi.mock("vue-router", () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock("@/stores/auth", () => ({
  useAuthStore: () => ({
    getPINLockoutStatus: vi.fn().mockResolvedValue({ locked: false, retryAfter: 0 }),
    loginWithPIN: vi.fn().mockResolvedValue({ ok: false, retryAfter: 0 }),
  }),
}));
vi.mock("../../wailsjs/go/main/App", () => ({
  GetAppVersion: vi.fn().mockResolvedValue("1.0.0"),
  ListBranches: vi.fn().mockResolvedValue({ branches: [] }),
  SwitchBranch: vi.fn(),
}));
vi.mock("../../wailsjs/runtime/runtime", () => ({ WindowReloadApp: vi.fn() }));

import LoginForm from "./LoginForm.vue";

let wrapper: VueWrapper<any> | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
});

async function mountForm() {
  wrapper = mount(LoginForm, { global: { plugins: [ui] }, attachTo: document.body });
  await flushPromises();
  return wrapper;
}
const key = (label: string) => wrapper!.find(`[aria-label="${label}"]`);

describe("PIN keypad colours", () => {
  it("C is a solid red key", async () => {
    await mountForm();
    const c = key("Clear PIN");
    expect(c.text()).toBe("C");
    expect(c.classes()).toContain("k-btn--variant-flat");
    expect(c.attributes("style") ?? "").toMatch(/error/);
  });

  it("backspace is a soft red key, apart from the solid C", async () => {
    await mountForm();
    const back = key("Delete last digit");
    expect(back.classes()).toContain("k-btn--variant-tonal");
    expect(back.attributes("style") ?? "").toMatch(/error/);
    expect(back.classes()).not.toContain("k-btn--variant-flat");
  });

  it("the digits stay plain grey keys", async () => {
    await mountForm();
    for (const d of ["1", "5", "9", "0"]) {
      const k = key(`Enter ${d}`);
      expect(k.classes()).toContain("k-btn--variant-tonal");
      expect(k.attributes("style") ?? "").not.toMatch(/error/);
    }
  });

  it("both red keys are off until a digit is entered, then work", async () => {
    await mountForm();
    expect(key("Clear PIN").attributes("disabled")).toBeDefined();
    expect(key("Delete last digit").attributes("disabled")).toBeDefined();
    await key("Enter 4").trigger("click");
    expect(key("Clear PIN").attributes("disabled")).toBeUndefined();
    expect(key("Delete last digit").attributes("disabled")).toBeUndefined();
    await key("Clear PIN").trigger("click");
    expect(key("Clear PIN").attributes("disabled")).toBeDefined();
  });
});
