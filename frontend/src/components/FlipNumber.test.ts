// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import FlipNumber from "./FlipNumber.vue";

describe("FlipNumber", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("shows the number with no flaps at rest", () => {
    const w = mount(FlipNumber, { props: { value: 7 } });
    expect(w.element.getAttribute("aria-label")).toBe("7");
    expect(w.findAll(".flip-flap")).toHaveLength(0);
    expect(w.findAll(".flip-num").map((n) => n.text())).toEqual(["7", "7"]);
  });

  it("flips from the old number to the new one, then settles", async () => {
    const w = mount(FlipNumber, { props: { value: 7 } });
    await w.setProps({ value: 8 });
    // Top flap folds away showing the old number; bottom flap brings the new.
    expect(w.find(".flip-flap-top").text()).toBe("7");
    expect(w.find(".flip-flap-bottom").text()).toBe("8");
    // Underneath: new on top, old still on the bottom until the flap lands.
    const [top, bottom] = w.findAll(".flip-half:not(.flip-flap) .flip-num");
    expect(top!.text()).toBe("8");
    expect(bottom!.text()).toBe("7");
    vi.advanceTimersByTime(700);
    await w.vm.$nextTick();
    expect(w.findAll(".flip-flap")).toHaveLength(0);
    expect(w.findAll(".flip-num").map((n) => n.text())).toEqual(["8", "8"]);
  });

  it("flips down too, and on rapid taps flips from the last shown number", async () => {
    const w = mount(FlipNumber, { props: { value: 3 } });
    await w.setProps({ value: 2 });
    expect(w.find(".flip-flap-top").text()).toBe("3");
    await w.setProps({ value: 1 });
    expect(w.find(".flip-flap-top").text()).toBe("2");
    expect(w.find(".flip-flap-bottom").text()).toBe("1");
    vi.advanceTimersByTime(700);
    await w.vm.$nextTick();
    expect(w.findAll(".flip-num").map((n) => n.text())).toEqual(["1", "1"]);
  });

  it("marks the card up or down while it flips", async () => {
    const w = mount(FlipNumber, { props: { value: 5 } });
    expect(w.classes()).not.toContain("flip--up");
    await w.setProps({ value: 6 });
    expect(w.classes()).toContain("flip--up");
    vi.advanceTimersByTime(700);
    await w.vm.$nextTick();
    expect(w.classes()).not.toContain("flip--up");
    await w.setProps({ value: 5 });
    expect(w.classes()).toContain("flip--down");
  });

  it("keeps the highlight on for its whole 0.6s, not just the fold", async () => {
    const w = mount(FlipNumber, { props: { value: 5 } });
    await w.setProps({ value: 6 });
    vi.advanceTimersByTime(500);
    await w.vm.$nextTick();
    expect(w.classes()).toContain("flip--up");
    vi.advanceTimersByTime(200);
    await w.vm.$nextTick();
    expect(w.classes()).not.toContain("flip--up");
  });

  it("restarts the highlight on every tap, so fast taps each show it", async () => {
    const w = mount(FlipNumber, { props: { value: 5 } });
    await w.setProps({ value: 6 });
    const first = w.find(".flip-wash").element;
    const firstClass = w.classes().filter((c) => c === "flip--odd" || c === "flip--even");
    vi.advanceTimersByTime(100); // tapped again before the first one has finished
    await w.setProps({ value: 7 });
    // A new wash element, and the card's animation swapped to its twin, which
    // is what makes the browser start both again from the beginning.
    expect(w.find(".flip-wash").element).not.toBe(first);
    const secondClass = w.classes().filter((c) => c === "flip--odd" || c === "flip--even");
    expect(secondClass).not.toEqual(firstClass);
    await w.setProps({ value: 8 });
    expect(w.classes().filter((c) => c === "flip--odd" || c === "flip--even")).toEqual(firstClass);
    expect(w.classes()).toContain("flip--up");
  });
});
