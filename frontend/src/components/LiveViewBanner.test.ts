// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mount, type VueWrapper } from "@vue/test-utils";
import { nextTick } from "vue";
import LiveViewBanner from "./LiveViewBanner.vue";
import { liveViewState } from "@/lib/liveView";

let wrapper: VueWrapper | null = null;
beforeEach(() => {
  vi.useFakeTimers();
  liveViewState.active = false;
  liveViewState.control = false;
  liveViewState.controlledUntil = 0;
});
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  vi.useRealTimers();
});

describe("LiveViewBanner", () => {
  it("shows nothing while nobody watches", () => {
    wrapper = mount(LiveViewBanner);
    expect(wrapper.find("[data-test=live-viewed]").exists()).toBe(false);
    expect(wrapper.find("[data-test=live-controlled]").exists()).toBe(false);
  });

  it("marks the till as viewed while a session runs", async () => {
    wrapper = mount(LiveViewBanner);
    liveViewState.active = true;
    await nextTick();
    expect(wrapper.find("[data-test=live-viewed]").text()).toBe("Being viewed");
  });

  it("shows a clear bar after a watcher acts, then goes back to the small mark", async () => {
    wrapper = mount(LiveViewBanner);
    liveViewState.active = true;
    liveViewState.controlledUntil = Date.now() + 3000;
    await nextTick();
    expect(wrapper.find("[data-test=live-controlled]").text()).toBe("Kashi is controlling this screen");
    expect(wrapper.find("[data-test=live-viewed]").exists()).toBe(false);

    vi.advanceTimersByTime(3600);
    await nextTick();
    expect(wrapper.find("[data-test=live-controlled]").exists()).toBe(false);
    expect(wrapper.find("[data-test=live-viewed]").exists()).toBe(true);
  });

  it("the bar goes at once when control is taken away or the session stops", async () => {
    wrapper = mount(LiveViewBanner);
    liveViewState.active = true;
    liveViewState.controlledUntil = Date.now() + 8000;
    await nextTick();
    liveViewState.controlledUntil = 0;
    await nextTick();
    expect(wrapper.find("[data-test=live-controlled]").exists()).toBe(false);

    liveViewState.controlledUntil = Date.now() + 8000;
    liveViewState.active = false;
    await nextTick();
    expect(wrapper.find("[data-test=live-controlled]").exists()).toBe(false);
    expect(wrapper.find("[data-test=live-viewed]").exists()).toBe(false);
  });
});
