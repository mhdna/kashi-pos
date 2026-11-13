// @vitest-environment happy-dom
//
// The item details' note: tapping it opens the on-screen keyboard, whose keys
// type into the note like a sentence (a capital only at the start and after a
// full stop), and the keyboard goes when the note loses focus.
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

vi.mock("../../wailsjs/go/main/App", () => ({
  ListCategoryIcons: vi.fn().mockResolvedValue([]),
  ListSalespersons: vi.fn().mockResolvedValue([]),
}));
vi.mock("@/lib/display", async () => {
  const { ref } = await import("vue");
  return { animationsEnabled: ref(false) };
});

import InvoiceItems from "./InvoiceItems.vue";

let wrapper: VueWrapper<any> | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  document.body.innerHTML = "";
});

const shirt = (over: Record<string, unknown> = {}) => ({
  name: "Shirt", code: "SH1", barcode: "111", price: 50, originalPrice: 50, originalDiscount: 0, qty: 1, discount: 0, ...over,
});

async function openDetails(line = shirt()) {
  wrapper = mount(InvoiceItems, { global: { plugins: [ui] }, attachTo: document.body });
  await flushPromises();
  const vm = wrapper.vm as any;
  vm.addItem(line);
  await flushPromises();
  await wrapper.find(".invoice-item-details-trigger").trigger("click");
  await flushPromises();
  return vm;
}
const note = () => document.body.querySelector<HTMLTextAreaElement>("[data-test=line-note]")!;
const keyboard = () => document.body.querySelector<HTMLElement>("[data-test=line-note-keyboard]");
const press = async (label: string) => {
  const key = Array.from(keyboard()!.querySelectorAll<HTMLElement>("button")).find((b) => b.textContent?.trim() === label);
  if (!key) throw new Error(`no key ${label}`);
  key.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  await flushPromises();
};
const typeKeys = async (text: string) => {
  for (const ch of text) await press(ch === " " ? "Space" : ch.toLowerCase());
};
const focus = async () => {
  note().dispatchEvent(new FocusEvent("focus"));
  await flushPromises();
};

describe("item note on-screen keyboard", () => {
  it("shows no keyboard until the note is tapped", async () => {
    await openDetails();
    expect(note()).toBeTruthy();
    expect(keyboard()).toBeNull();
    await focus();
    expect(keyboard()).toBeTruthy();
  });

  it("is its own panel on the body, above the dialog, not inside the dialog card", async () => {
    await openDetails();
    await focus();
    const kb = keyboard()!;
    const dialogEl = document.body.querySelector<HTMLElement>(".k-dialog")!;
    const content = dialogEl.querySelector(".k-overlay__content")!;
    expect(content.contains(kb)).toBe(false);
    expect(dialogEl.contains(kb)).toBe(false);
    expect(kb.parentElement).toBe(document.body);
    expect(Number(kb.style.zIndex)).toBeGreaterThan(Number(dialogEl.style.zIndex));
  });

  it("floats over the dialog without moving it", async () => {
    await openDetails();
    const dialogEl = document.body.querySelector<HTMLElement>(".k-dialog")!;
    const before = { cls: dialogEl.className, style: dialogEl.getAttribute("style") };
    await focus();
    expect(keyboard()).toBeTruthy();
    expect(dialogEl.className).toBe(before.cls);
    expect(dialogEl.getAttribute("style")).toBe(before.style);
    expect(dialogEl.style.paddingBottom).toBe("");
  });

  it("has a close button that puts the keyboard away and keeps the note", async () => {
    const vm = await openDetails();
    await focus();
    await typeKeys("hi");
    const close = keyboard()!.querySelector<HTMLElement>("[data-test=line-note-keyboard-close]");
    expect(close?.getAttribute("aria-label")).toBe("Close keyboard");
    // The same Close button as the product search keyboard: red, tonal, an X and the word.
    expect(close?.textContent?.trim()).toBe("Close");
    expect(close?.className).toContain("k-btn--variant-tonal");
    expect(close?.getAttribute("style") ?? "").toMatch(/error/);
    expect(close?.querySelector(".k-icon")).toBeTruthy();
    close!.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();
    expect(keyboard()).toBeNull();
    expect(vm.items[0].note).toBe("Hi");
    // Tapping the note again brings it back.
    await focus();
    expect(keyboard()).toBeTruthy();
  });

  it("hides the keyboard when the note loses focus", async () => {
    await openDetails();
    await focus();
    note().dispatchEvent(new FocusEvent("blur"));
    await flushPromises();
    expect(keyboard()).toBeNull();
  });

  it("types into the note as a sentence: capital at the start and after a full stop", async () => {
    const vm = await openDetails();
    await focus();
    await typeKeys("gift wrap. call me");
    expect(vm.items[0].note).toBe("Gift wrap. Call me");
  });

  it("has digits, a full stop and a comma, and backspace and clear", async () => {
    const vm = await openDetails();
    await focus();
    await typeKeys("size 42");
    await press(",");
    await press(".");
    expect(vm.items[0].note).toBe("Size 42,.");
    const back = keyboard()!.querySelector<HTMLElement>(".inline-keyboard-backspace")!;
    back.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();
    expect(vm.items[0].note).toBe("Size 42,");
    await press("C");
    expect(vm.items[0].note).toBe("");
  });

  it("continues a note that was already there", async () => {
    const vm = await openDetails(shirt({ note: "Gift wrap" }));
    expect(note().value).toBe("Gift wrap");
    await focus();
    await typeKeys(" x");
    expect(vm.items[0].note).toBe("Gift wrap x");
  });

  it("counts the characters against the limit", async () => {
    await openDetails(shirt({ note: "abc" }));
    expect(document.body.querySelector(".line-note-count")!.textContent!.trim()).toBe("3 / 2000");
  });

  it("stops at the limit", async () => {
    const vm = await openDetails(shirt({ note: "a".repeat(2000) }));
    await focus();
    await press("b");
    expect(vm.items[0].note).toHaveLength(2000);
  });
});
