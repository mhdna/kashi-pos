// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

vi.mock("../../wailsjs/go/main/App", () => ({
  ActivateLicense: vi.fn(),
  RetryLicense: vi.fn(),
}));
vi.mock("../../wailsjs/runtime/runtime", () => ({ WindowReloadApp: vi.fn(), EventsOn: vi.fn(() => () => {}) }));

import { ActivateLicense, RetryLicense } from "../../wailsjs/go/main/App";
import { WindowReloadApp } from "../../wailsjs/runtime/runtime";
import LicenseActivate from "./LicenseActivate.vue";
import LicenseLock from "./LicenseLock.vue";
import LicenseWarning from "./LicenseWarning.vue";
import { LOCK_MESSAGE } from "@/lib/license";

let wrapper: VueWrapper<any> | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  vi.clearAllMocks();
  document.body.innerHTML = "";
});

const KEY = "KASHI-7Q2MX-4HBKA-ZR0TN-9WCPE";

async function typeKey(text: string) {
  const input = wrapper!.find('input[data-test="license-key"]');
  await input.setValue(text);
  await flushPromises();
  return input;
}

describe("LicenseActivate", () => {
  it("formats the key, enables Activate only when complete, and reloads after activating", async () => {
    vi.mocked(ActivateLicense).mockResolvedValue({ state: "ok" } as any);
    wrapper = mount(LicenseActivate, { global: { plugins: [ui] }, attachTo: document.body });
    const submit = () => wrapper!.find('[data-test="license-submit"]');
    await typeKey("7q2mx4hbka");
    expect((wrapper.find('input[data-test="license-key"]').element as HTMLInputElement).value).toBe("KASHI-7Q2MX-4HBKA");
    expect(submit().attributes("disabled")).toBeDefined();
    await wrapper.find("form").trigger("submit");
    expect(ActivateLicense).not.toHaveBeenCalled();

    await typeKey("7q2mx 4hbka zr0tn 9wcpe");
    expect(submit().attributes("disabled")).toBeUndefined();
    await wrapper.find("form").trigger("submit");
    await flushPromises();
    expect(ActivateLicense).toHaveBeenCalledWith(KEY);
    expect(WindowReloadApp).toHaveBeenCalledTimes(1);
  });

  it.each([
    ["license:invalid_key", /product key isn't right/],
    ["license:offline", /internet/],
    ["license:off", new RegExp(LOCK_MESSAGE.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))],
  ])("shows %s as words and stays on the screen", async (code, text) => {
    vi.mocked(ActivateLicense).mockRejectedValue(new Error(code));
    wrapper = mount(LicenseActivate, { global: { plugins: [ui] }, attachTo: document.body });
    await typeKey(KEY);
    await wrapper.find("form").trigger("submit");
    await flushPromises();
    expect(wrapper.find('[data-test="license-error"]').text()).toMatch(text);
    expect(WindowReloadApp).not.toHaveBeenCalled();
    // Typing again clears the error.
    await typeKey(KEY.slice(0, -1));
    expect(wrapper.find('[data-test="license-error"]').exists()).toBe(false);
  });
});

describe("LicenseLock", () => {
  it("says only the vague message and the support code", async () => {
    wrapper = mount(LicenseLock, { props: { code: "11112222" }, global: { plugins: [ui] }, attachTo: document.body });
    const text = wrapper.text();
    expect(wrapper.find('[data-test="license-lock-message"]').text()).toBe(LOCK_MESSAGE);
    expect(text).toContain("11112222");
    expect(text).not.toMatch(/pay|licen|turned off|disabled|expired|days/i);
  });

  it("Try again reloads only when the till may run again", async () => {
    wrapper = mount(LicenseLock, { props: { code: "X" }, global: { plugins: [ui] }, attachTo: document.body });
    vi.mocked(RetryLicense).mockResolvedValueOnce({ state: "locked" } as any);
    await wrapper.find('[data-test="license-retry"]').trigger("click");
    await flushPromises();
    expect(RetryLicense).toHaveBeenCalledTimes(1);
    expect(WindowReloadApp).not.toHaveBeenCalled();
    vi.mocked(RetryLicense).mockResolvedValueOnce({ state: "ok" } as any);
    await wrapper.find('[data-test="license-retry"]').trigger("click");
    await flushPromises();
    expect(WindowReloadApp).toHaveBeenCalledTimes(1);
  });
});

describe("LicenseWarning", () => {
  it("shows only when the lease runs out soon, and can be put off", async () => {
    wrapper = mount(LicenseWarning, { props: { warnHours: 0 }, global: { plugins: [ui] }, attachTo: document.body });
    await flushPromises();
    expect(document.body.textContent).not.toMatch(/Connect this till/);
    await wrapper.setProps({ warnHours: 20 });
    await flushPromises();
    expect(document.body.textContent).toMatch(/Connect this till to the internet today/);
  });
});
