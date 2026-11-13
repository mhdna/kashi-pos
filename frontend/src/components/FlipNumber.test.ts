// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import FlipNumber from "./FlipNumber.vue";

// What each digit card shows, and which are turning right now.
const cards = (w: ReturnType<typeof mount>) => w.findAll("[data-test=flip-digit]");
const shown = (w: ReturnType<typeof mount>) =>
  cards(w).map((c) => c.findAll(".flip-half:not(.flip-flap) .flip-num").map((n) => n.text()));
const turning = (w: ReturnType<typeof mount>) => cards(w).map((c) => c.find(".flip-flap").exists());
const wait = async (w: ReturnType<typeof mount>, ms: number) => {
  vi.advanceTimersByTime(ms);
  await w.vm.$nextTick();
};

describe("FlipNumber", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("is one card per digit, with the whole number as its label", () => {
    const w = mount(FlipNumber, { props: { value: 477 } });
    expect(w.attributes("aria-label")).toBe("477");
    expect(shown(w)).toEqual([["4", "4"], ["7", "7"], ["7", "7"]]);
    expect(turning(w)).toEqual([false, false, false]);
  });

  it("turns only the digit that changed", async () => {
    const w = mount(FlipNumber, { props: { value: 477 } });
    await w.setProps({ value: 478 });
    expect(turning(w)).toEqual([false, false, true]);
    const last = cards(w)[2]!;
    expect(last.find(".flip-flap-top").text()).toBe("7"); // folds away
    expect(last.find(".flip-flap-bottom").text()).toBe("8"); // unfolds
    // Underneath: new on top, old on the bottom until the flap lands.
    expect(shown(w)[2]).toEqual(["8", "7"]);
    await wait(w, 700);
    expect(turning(w)).toEqual([false, false, false]);
    expect(shown(w)).toEqual([["4", "4"], ["7", "7"], ["8", "8"]]);
  });

  it("turns every digit that changes, and only those", async () => {
    const w = mount(FlipNumber, { props: { value: 479 } });
    await w.setProps({ value: 480 });
    expect(turning(w)).toEqual([false, true, true]);
    await wait(w, 700);
    await w.setProps({ value: 580 });
    expect(turning(w)).toEqual([true, false, false]);
  });

  it("a digit that comes out the same does not turn (4 to 4 in 477 to 487 is just the middle)", async () => {
    const w = mount(FlipNumber, { props: { value: 477 } });
    await w.setProps({ value: 487 });
    expect(turning(w)).toEqual([false, true, false]);
  });

  it("grows and shrinks by a card at the left, keeping the units card", async () => {
    const w = mount(FlipNumber, { props: { value: 99 } });
    const units = cards(w)[1]!.element;
    await w.setProps({ value: 100 });
    expect(cards(w)).toHaveLength(3);
    expect(w.attributes("aria-label")).toBe("100");
    expect(cards(w)[2]!.element).toBe(units); // same card, now turning 9 to 0
    expect(turning(w)).toEqual([false, true, true]); // new 1 appears, 9->0, 9->0
    await wait(w, 700);
    await w.setProps({ value: 99 });
    expect(cards(w)).toHaveLength(2);
    expect(cards(w)[1]!.element).toBe(units);
  });

  it("marks each turning card up or down by which way the count went", async () => {
    const w = mount(FlipNumber, { props: { value: 479 } });
    await w.setProps({ value: 480 }); // 9 -> 0 is still "up" for the count
    expect(cards(w)[2]!.classes()).toContain("flip--up");
    expect(cards(w)[1]!.classes()).toContain("flip--up");
    expect(cards(w)[0]!.classes()).not.toContain("flip--up");
    await wait(w, 700);
    await w.setProps({ value: 479 });
    expect(cards(w)[2]!.classes()).toContain("flip--down");
  });

  it("keeps the highlight on for its whole 0.6s, then ends it", async () => {
    const w = mount(FlipNumber, { props: { value: 5 } });
    await w.setProps({ value: 6 });
    await wait(w, 500);
    expect(cards(w)[0]!.classes()).toContain("flip--up");
    await wait(w, 200);
    expect(cards(w)[0]!.classes()).not.toContain("flip--up");
  });

  it("restarts a digit's animation on every tap, so fast taps each show it", async () => {
    const w = mount(FlipNumber, { props: { value: 5 } });
    const twin = () => cards(w)[0]!.classes().filter((c) => c === "flip--odd" || c === "flip--even");
    await w.setProps({ value: 6 });
    const first = twin();
    const shade = cards(w)[0]!.find(".flip-shade").element;
    await wait(w, 100); // tapped again before the first has finished
    await w.setProps({ value: 7 });
    expect(twin()).not.toEqual(first); // swapped to the twin animation
    expect(cards(w)[0]!.find(".flip-shade").element).not.toBe(shade); // and a fresh shade
    await w.setProps({ value: 8 });
    expect(twin()).toEqual(first);
  });
});
