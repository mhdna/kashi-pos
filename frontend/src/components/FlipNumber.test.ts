// @vitest-environment happy-dom
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
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

  it("restarts on every tap: a fresh highlight bar and fresh shade, so fast taps each show it", async () => {
    const w = mount(FlipNumber, { props: { value: 5 } });
    await w.setProps({ value: 6 });
    const bar = w.find("[data-test=flip-tint]").element;
    const shade = cards(w)[0]!.find(".flip-shade").element;
    await wait(w, 100); // tapped again before the first has finished
    await w.setProps({ value: 7 });
    expect(w.find("[data-test=flip-tint]").element).not.toBe(bar);
    expect(cards(w)[0]!.find(".flip-shade").element).not.toBe(shade);
    expect(w.findAll("[data-test=flip-tint]")).toHaveLength(1);
  });

  it("the ring and the colour are the shared bar's alone: no turning card draws its own", () => {
    const digit = readFileSync(resolve(process.cwd(), "src/components/FlipDigit.vue"), "utf8");
    const number = readFileSync(resolve(process.cwd(), "src/components/FlipNumber.vue"), "utf8");
    // A card has no ring or inner shadow of its own while it turns...
    expect(digit).not.toMatch(/@keyframes flip-pop/);
    expect(digit).toMatch(/\.flip--up,\s*\.flip--down \{\s*box-shadow: none;/);
    // ...the one bar behind them and the one ring over them do that, as single pieces.
    expect(number).toMatch(/@keyframes flip-ring-pop/);
    expect(number).toMatch(/animation-name: flip-tint-fade, flip-tint-pop/);
    // The ring is above the cards (z-index 3 over their 1), so a lone turning digit
    // keeps its whole outline, its left side included.
    expect(number).toMatch(/\.flip-ring \{\s*z-index: 3;/);
    expect(digit).toMatch(/\.flip \{[^}]*z-index: 1;/);
  });

  describe("one shared highlight behind the digits that change", () => {
    const tint = (w: ReturnType<typeof mount>) => w.findAll("[data-test=flip-tint]");
    const span = (w: ReturnType<typeof mount>) => {
      const el = tint(w)[0]!;
      return [Number(el.attributes("data-from")), Number(el.attributes("data-count"))];
    };

    it("has none at rest", () => {
      expect(tint(mount(FlipNumber, { props: { value: 477 } }))).toHaveLength(0);
    });

    it("spans only the card that turned for a plain step", async () => {
      const w = mount(FlipNumber, { props: { value: 477 } });
      await w.setProps({ value: 478 });
      expect(tint(w)).toHaveLength(1);
      expect(span(w)).toEqual([2, 1]);
    });

    it("is ONE highlight across every card a carry turns, not one per card", async () => {
      const w = mount(FlipNumber, { props: { value: 479 } });
      await w.setProps({ value: 480 });
      expect(turning(w)).toEqual([false, true, true]);
      expect(tint(w)).toHaveLength(1);
      expect(span(w)).toEqual([1, 2]);
      await wait(w, 700);
      await w.setProps({ value: 500 });
      expect(span(w)).toEqual([0, 3]);
    });

    it("covers the new card when the number grows", async () => {
      const w = mount(FlipNumber, { props: { value: 99 } });
      await w.setProps({ value: 100 });
      expect(span(w)).toEqual([0, 3]);
    });

    it("ends with the flip, and a fast second tap gets a fresh one", async () => {
      const w = mount(FlipNumber, { props: { value: 5 } });
      await w.setProps({ value: 6 });
      const first = tint(w)[0]!.element;
      await wait(w, 100);
      await w.setProps({ value: 7 });
      expect(tint(w)[0]!.element).not.toBe(first);
      await wait(w, 700);
      expect(tint(w)).toHaveLength(0);
    });

    it("is slate blue going up and reddish going down: colours in the source", () => {
    const css = readFileSync(resolve(process.cwd(), "src/components/FlipNumber.vue"), "utf8");
    const hex = (m: RegExpMatchArray) => [1, 3, 5].map((i) => parseInt(m[1]!.slice(i - 1, i + 1), 16)) as [number, number, number];
    const up = hex(css.match(/\.flip-number \{[^}]*--tint: #([0-9a-f]{6})/)!);
    const down = hex(css.match(/\.flip-number--down \{[^}]*--tint: #([0-9a-f]{6})/)!);
    expect(up[2]).toBeGreaterThan(up[0]); // blue leads red: a blue, not the old amber
    expect(up[2]).toBeGreaterThan(up[1]);
    expect(up[2]).toBeGreaterThan(160); // a strong blue, not the muted slate
    expect(down[0]).toBeGreaterThan(160); // and a strong red
    expect(down[0]).toBeGreaterThan(down[2]); // red leads: a reddish one, not the old slate blue
    expect(down[0]).toBeGreaterThan(down[1]);
    // The rings are the same families: blue going up, red going down.
    const flash = (re: RegExp) => css.match(re)!.slice(1, 4).map(Number) as [number, number, number];
    const upRing = flash(/\.flip-number \{[^}]*--flash: (\d+), (\d+), (\d+)/);
    const downRing = flash(/\.flip-number--down \{[^}]*--flash: (\d+), (\d+), (\d+)/);
    expect(upRing[2]).toBeGreaterThan(upRing[0]);
    expect(downRing[0]).toBeGreaterThan(downRing[2]);
    // And a flap that carries the tint falls back to blue, not amber.
    const digit = readFileSync(resolve(process.cwd(), "src/components/FlipDigit.vue"), "utf8");
    expect(digit).toMatch(/var\(--tint, #3a68c0\)/);
  });

  it("tells the direction: up and down classes on the number", async () => {
      const w = mount(FlipNumber, { props: { value: 5 } });
      expect(w.classes()).toContain("flip-number--up");
      await w.setProps({ value: 4 });
      expect(w.classes()).toContain("flip-number--down");
    });
  });
});

describe("FlipNumber ring", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("is its own layer over the same run as the bar, so a lone turning digit keeps its whole outline", async () => {
    const w = mount(FlipNumber, { props: { value: 477 } });
    expect(w.find("[data-test=flip-ring]").exists()).toBe(false);
    await w.setProps({ value: 478 }); // a lone digit turns
    const bar = w.find("[data-test=flip-tint]");
    const ring = w.find("[data-test=flip-ring]");
    expect(ring.exists()).toBe(true);
    expect(ring.attributes("style")).toBe(bar.attributes("style")); // same --from and --count
    vi.advanceTimersByTime(700);
    await w.vm.$nextTick();
    expect(w.find("[data-test=flip-ring]").exists()).toBe(false);
    await w.setProps({ value: 480 }); // two digits turn
    expect(w.findAll("[data-test=flip-ring]")).toHaveLength(1); // one ring for the whole run
    expect(w.find("[data-test=flip-ring]").attributes("style")).toContain("--count: 2");
  });

  it("restarts with every tap", async () => {
    const w = mount(FlipNumber, { props: { value: 5 } });
    await w.setProps({ value: 6 });
    const first = w.find("[data-test=flip-ring]").element;
    vi.advanceTimersByTime(100);
    await w.setProps({ value: 7 });
    expect(w.find("[data-test=flip-ring]").element).not.toBe(first);
  });
});

describe("FlipNumber: only the digits that change grow", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  const grown = (w: ReturnType<typeof mount>) =>
    w.findAll("[data-test=flip-digit]").map((c) => c.classes().some((k) => k === "flip--even" || k === "flip--odd"));
  const origin = (w: ReturnType<typeof mount>, i: number) =>
    Number(/--oxc: ([\d.-]+)/.exec(w.findAll("[data-test=flip-digit]")[i]!.attributes("style") ?? "")![1]);

  it("a lone turning digit grows by itself, the others stay as they are", async () => {
    const w = mount(FlipNumber, { props: { value: 477 } });
    expect(grown(w)).toEqual([false, false, false]);
    await w.setProps({ value: 478 });
    expect(grown(w)).toEqual([false, false, true]);
  });

  it("a run that turns together grows together, from its own middle", async () => {
    const w = mount(FlipNumber, { props: { value: 479 } });
    await w.setProps({ value: 480 }); // the last two turn: cards 1 and 2
    expect(grown(w)).toEqual([false, true, true]);
    // The run's middle is a card and a half... measured from each card's left edge:
    // card 1 grows from 1 card to its right, card 2 from 1 card to its left.
    expect(origin(w, 1)).toBe(1);
    expect(origin(w, 2)).toBe(0);
    // All three turning (499 -> 500): the middle is card 1.5.
    const all = mount(FlipNumber, { props: { value: 499 } });
    await all.setProps({ value: 500 });
    expect(grown(all)).toEqual([true, true, true]);
    expect([0, 1, 2].map((i) => origin(all, i))).toEqual([1.5, 0.5, -0.5]);
  });

  it("stops with the highlight", async () => {
    const w = mount(FlipNumber, { props: { value: 5 } });
    await w.setProps({ value: 6 });
    expect(grown(w)).toEqual([true]);
    vi.advanceTimersByTime(700);
    await w.vm.$nextTick();
    expect(grown(w)).toEqual([false]);
  });

  it("restarts on every tap: the card swaps to its twin animation", async () => {
    const w = mount(FlipNumber, { props: { value: 5 } });
    const twin = () => w.find("[data-test=flip-digit]").classes().filter((c) => c === "flip--even" || c === "flip--odd");
    await w.setProps({ value: 6 });
    const first = twin();
    vi.advanceTimersByTime(100);
    await w.setProps({ value: 7 });
    expect(twin()).not.toEqual(first);
    await w.setProps({ value: 8 });
    expect(twin()).toEqual(first);
  });

  it("grows a lot going up and only a little going down", () => {
    const css = readFileSync(resolve(process.cwd(), "src/components/FlipNumber.vue"), "utf8");
    const up = Number(css.match(/\.flip-number \{[^}]*--grow: ([\d.]+)/)![1]);
    const down = Number(css.match(/\.flip-number--down \{[^}]*--grow: ([\d.]+)/)![1]);
    expect(up).toBeGreaterThan(1.15);
    expect(down).toBeGreaterThan(1);
    expect(down).toBeLessThan(up);
  });

  it("the box is the highlight's container: relative, with the highlight laid inside it (not over the sidebar)", () => {
    const css = readFileSync(resolve(process.cwd(), "src/components/FlipNumber.vue"), "utf8");
    // The bar and ring are absolutely placed, so something around them must be positioned.
    expect(css).toMatch(/\.flip-tint,\s*\.flip-ring \{[^}]*position: absolute/);
    expect(css).toMatch(/\.flip-number \{[^}]*position: relative/);
    // And they are inside the number's own box, a fixed 40px tall: top and bottom are its padding.
    expect(css).toMatch(/\.flip-tint,\s*\.flip-ring \{[^}]*top: var\(--pad\)/);
    expect(css).toMatch(/\.flip-tint,\s*\.flip-ring \{[^}]*bottom: var\(--pad\)/);
    expect(css).toMatch(/\.flip-number \{[^}]*height: 40px/);
  });
});
