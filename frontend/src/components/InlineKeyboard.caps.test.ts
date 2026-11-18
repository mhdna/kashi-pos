// @vitest-environment happy-dom
//
// The "caps" keyboard for codes: a number row and capitals only.
import { afterEach, describe, expect, it } from "vitest";
import { mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";
import InlineKeyboard from "./InlineKeyboard.vue";

let wrapper: VueWrapper<any> | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
});

function keyboard(layout: "caps" | "text", value = "") {
  wrapper = mount(InlineKeyboard, {
    props: { modelValue: value, layout, maxLength: 12, "onUpdate:modelValue": (v: string) => wrapper?.setProps({ modelValue: v }) },
    global: { plugins: [ui] },
  });
  return wrapper;
}
const key = (w: VueWrapper<any>, label: string) => w.findAll(".inline-keyboard-key").find((k) => k.text().trim() === label)!;

describe("InlineKeyboard caps", () => {
  it("the reported case: a number row on top and every letter a capital", () => {
    const labels = keyboard("caps").findAll(".inline-keyboard-key").map((k) => k.text().trim());
    expect(labels.slice(0, 10)).toEqual(["1", "2", "3", "4", "5", "6", "7", "8", "9", "0"]);
    expect(labels.slice(10, 13)).toEqual(["Q", "W", "E"]);
    expect(labels.filter((l) => /[a-z]/.test(l))).toEqual([]);
  });
  it("types capitals and digits, never small letters, after the first key too", async () => {
    const w = keyboard("caps");
    for (const k of ["K", "7", "M", "4", "Q", "H"]) await key(w, k).trigger("click");
    expect(w.props("modelValue")).toBe("K7M4QH");
  });
  it("has no dash or space, and stops at its length", async () => {
    const w = keyboard("caps", "K7M4Q");
    const all = w.findAll("button").map((b) => b.text().trim());
    expect(all).not.toContain("-");
    expect(all).not.toContain("Space");
    await w.setProps({ maxLength: 6 });
    await key(w, "H").trigger("click");
    await key(w, "X").trigger("click");
    expect(w.props("modelValue")).toBe("K7M4QH");
  });

  it("puts C and backspace at the keyboard's right edge", () => {
    expect(keyboard("caps").find(".inline-keyboard-actions").classes()).toContain("inline-keyboard-actions--end");
    wrapper!.unmount();
    expect(keyboard("text").find(".inline-keyboard-actions").classes()).not.toContain("inline-keyboard-actions--end");
  });

  it("the text keyboard is as it was", async () => {
    const w = keyboard("text");
    const labels = w.findAll(".inline-keyboard-key").map((k) => k.text().trim());
    expect(labels[0]).toBe("q");
    expect(w.findAll("button").map((b) => b.text().trim())).toContain("Space");
    await key(w, "q").trigger("click");
    await key(w, "w").trigger("click");
    expect(w.props("modelValue")).toBe("Qw");
  });
});
