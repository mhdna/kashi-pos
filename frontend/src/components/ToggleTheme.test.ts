// @vitest-environment happy-dom
//
// The header's Theme button: switches the theme at once, saves it, and goes
// back if saving fails.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui, { useTheme } from "@/ui";

vi.mock("../../wailsjs/go/main/App", () => ({ SetTheme: vi.fn() }));

import { SetTheme } from "../../wailsjs/go/main/App";
import ToggleTheme from "./ToggleTheme.vue";

const setTheme = vi.mocked(SetTheme);
let wrapper: VueWrapper<any> | null = null;

beforeEach(() => {
  setTheme.mockReset();
  setTheme.mockResolvedValue(undefined as any);
  useTheme().change("light");
});
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
});

const mountIt = () => (wrapper = mount(ToggleTheme, { global: { plugins: [ui] } }));
const button = () => wrapper!.find("[data-test=theme-toggle]");

describe("theme toggle", () => {
  it("is just a moon icon in the light theme: no caption, no background", () => {
    mountIt();
    expect(wrapper!.text()).toBe("");
    expect(button().classes()).toContain("k-btn--variant-text");
    expect(button().attributes("aria-label")).toBe("Switch to dark theme");
    expect(button().find(".k-icon").exists()).toBe(true);
  });

  it("switches to dark, saves it, and offers the way back", async () => {
    mountIt();
    await button().trigger("click");
    expect(useTheme().name.value).toBe("dark");
    expect(setTheme).toHaveBeenCalledWith("dark");
    await flushPromises();
    expect(button().attributes("aria-label")).toBe("Switch to light theme");
  });

  it("switches back to light", async () => {
    useTheme().change("dark");
    mountIt();
    await button().trigger("click");
    await flushPromises();
    expect(useTheme().name.value).toBe("light");
    expect(setTheme).toHaveBeenCalledWith("light");
  });

  it("goes back to the old theme when saving fails", async () => {
    setTheme.mockRejectedValue(new Error("disk full"));
    mountIt();
    await button().trigger("click");
    await flushPromises();
    expect(useTheme().name.value).toBe("light");
  });

  it("ignores a second tap while the first is still saving", async () => {
    let finish!: () => void;
    setTheme.mockReturnValue(new Promise<void>((resolve) => (finish = resolve)) as any);
    mountIt();
    await button().trigger("click");
    await button().trigger("click");
    expect(setTheme).toHaveBeenCalledTimes(1);
    finish();
    await flushPromises();
    expect(useTheme().name.value).toBe("dark");
  });
});
