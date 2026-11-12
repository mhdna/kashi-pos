// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

const CreateBranch = vi.fn(async () => ({}));
const SwitchBranch = vi.fn(async () => ({}));
const WindowReloadApp = vi.fn();
vi.mock("../../wailsjs/go/main/App", () => ({ CreateBranch: (...a: unknown[]) => CreateBranch(...(a as [])), SwitchBranch: (...a: unknown[]) => SwitchBranch(...(a as [])) }));
vi.mock("../../wailsjs/runtime/runtime", () => ({ WindowReloadApp: () => WindowReloadApp() }));

import BranchSetup from "./BranchSetup.vue";

let wrapper: VueWrapper<any> | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  CreateBranch.mockReset().mockResolvedValue({});
  SwitchBranch.mockReset().mockResolvedValue({});
  WindowReloadApp.mockReset();
});

function show(knownBranches: { name: string; code: string }[] = [], startupError = "") {
  wrapper = mount(BranchSetup, { props: { knownBranches, startupError }, global: { plugins: [ui] }, attachTo: document.body });
  return wrapper;
}
const input = (sel: string) => wrapper!.find(`input[data-test="${sel}"]`);
async function tapEye(sel: string) {
  const eye = (input(sel).element.closest(".k-field") as HTMLElement).querySelector(".k-field__append") as HTMLElement;
  eye.click();
  await flushPromises();
}

describe("BranchSetup", () => {
  it("the reported case: the API key and server address are masked until the eye is tapped", async () => {
    show();
    await input("setup-key").setValue("test-key-not-real-0000000000000000");
    await input("setup-url").setValue("cloud.example.com/api");
    expect(input("setup-key").attributes("type")).toBe("password");
    expect(input("setup-url").attributes("type")).toBe("password");
    expect(wrapper!.text()).not.toContain("test-key-not-real");

    await tapEye("setup-key");
    expect(input("setup-key").attributes("type")).toBe("text");
    expect(input("setup-url").attributes("type")).toBe("password");
    await tapEye("setup-key");
    expect(input("setup-key").attributes("type")).toBe("password");
  });

  it("creates the branch with trimmed values and an upper-case code, then reloads", async () => {
    show();
    await input("setup-name").setValue(" Centro ");
    await input("setup-code").setValue("c-n");
    expect((input("setup-code").element as HTMLInputElement).value).toBe("CN");
    await input("setup-key").setValue(" key ");
    await input("setup-url").setValue(" cloud.example.com/api ");
    expect(wrapper!.find('[data-test="setup-hint"]').text()).toContain("sync with central");
    await wrapper!.find('[data-test="setup-form"]').trigger("submit");
    await flushPromises();
    expect(CreateBranch).toHaveBeenCalledWith("Centro", "CN", "key", "cloud.example.com/api");
    expect(WindowReloadApp).toHaveBeenCalled();
  });

  it("needs a name and a code before it calls the backend", async () => {
    show();
    await wrapper!.find('[data-test="setup-form"]').trigger("submit");
    await flushPromises();
    expect(CreateBranch).not.toHaveBeenCalled();
    expect(wrapper!.text()).toContain("Give the branch a name");
    expect(wrapper!.text()).toContain("short code");
  });

  it("warns when only the key is filled, and says standalone when nothing is", async () => {
    show();
    expect(wrapper!.find('[data-test="setup-hint"]').text()).toContain("on its own");
    await input("setup-key").setValue("k");
    expect(wrapper!.find('[data-test="setup-hint"]').classes()).toContain("setup-hint--warn");
  });

  it("shows the backend's error and does not reload", async () => {
    CreateBranch.mockRejectedValue(new Error('branch "CN" is already set up on this machine'));
    show();
    await input("setup-name").setValue("Centro");
    await input("setup-code").setValue("CN");
    await wrapper!.find('[data-test="setup-form"]').trigger("submit");
    await flushPromises();
    expect(wrapper!.find('[data-test="setup-error"]').text()).toContain("already set up");
    expect(WindowReloadApp).not.toHaveBeenCalled();
  });

  it("with branches already here: pick one, or set up another", async () => {
    show([{ name: "Centro", code: "CN" }, { name: "Hara", code: "HR" }], "the active branch's database is missing");
    expect(wrapper!.find('[data-test="setup-form"]').exists()).toBe(false);
    expect(wrapper!.find('[data-test="setup-startup-error"]').text()).toContain("missing");
    await wrapper!.find('[data-test="setup-branch-HR"]').trigger("click");
    await flushPromises();
    expect(SwitchBranch).toHaveBeenCalledWith("HR");
    expect(WindowReloadApp).toHaveBeenCalled();

    await wrapper!.find('[data-test="setup-add"]').trigger("click");
    expect(wrapper!.find('[data-test="setup-form"]').exists()).toBe(true);
  });
});
