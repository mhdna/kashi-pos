// @vitest-environment happy-dom
//
// "Unknown item": the price on a number pad, then the item's name on the
// on-screen keyboard; the sale's list and receipt say that name.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";
import UnknownPriceDialog from "./UnknownPriceDialog.vue";

let wrapper: VueWrapper<any> | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  document.body.innerHTML = "";
});

async function openDialog() {
  const w = mount(UnknownPriceDialog, {
    props: { modelValue: false, "onUpdate:modelValue": (v: boolean) => void w.setProps({ modelValue: v }) },
    global: { plugins: [ui] },
    attachTo: document.body,
  });
  wrapper = w;
  await w.setProps({ modelValue: true });
  await flushPromises();
  return w;
}
const body = () => document.body;
const q = (s: string) => body().querySelector<HTMLElement>(s);
const click = async (el: Element | null | undefined) => {
  el!.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  await flushPromises();
};
const button = (label: string) =>
  Array.from(body().querySelectorAll<HTMLElement>("button")).find((b) => b.textContent?.trim() === label) ?? null;
const typePrice = async (digits: string) => {
  for (const d of digits) await click(button(d));
};
const typeName = async (text: string) => {
  for (const ch of text) await click(button(ch === " " ? "Space" : ch.toLowerCase()));
};
const next = () => q("[data-test=unknown-next]") as HTMLButtonElement;
const add = () => q("[data-test=unknown-add]") as HTMLButtonElement;
const key = async (k: string) => {
  window.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true }));
  await flushPromises();
};

describe("unknown item dialog", () => {
  it("starts on the price: no name step yet, and Next waits for a price", async () => {
    await openDialog();
    expect(q("[data-testid=unknown-price]")?.textContent?.trim()).toBe("$0");
    expect(q("[data-testid=unknown-name]")).toBeNull();
    expect(next().disabled).toBe(true);
    await typePrice("25");
    expect(q("[data-testid=unknown-price]")?.textContent?.trim()).toBe("$25");
    expect(next().disabled).toBe(false);
  });

  it("asks for the name after the price, with the keyboard, and shows the price it is for", async () => {
    await openDialog();
    await typePrice("25");
    await click(next());
    expect(body().textContent).toContain("Name the item");
    expect(q("[data-testid=unknown-name]")?.textContent?.trim()).toBe("Name"); // the placeholder
    expect(q(".inline-keyboard")).toBeTruthy();
    expect(body().textContent).toContain("$25");
  });

  it("needs a name before Add, and Add sends the price and the name, then closes", async () => {
    const w = await openDialog();
    await typePrice("12.5");
    await click(next());
    expect(add().disabled).toBe(true);
    await typeName("red scarf");
    expect(q("[data-testid=unknown-name]")?.textContent?.trim()).toBe("Red Scarf");
    expect(add().disabled).toBe(false);
    await click(add());
    expect(w.emitted("add")).toEqual([[12.5, "Red Scarf"]]);
    expect(w.props("modelValue")).toBe(false);
  });

  it("a name of only spaces is no name", async () => {
    await openDialog();
    await typePrice("5");
    await click(next());
    await click(button("Space"));
    await click(button("Space"));
    expect(add().disabled).toBe(true);
  });

  it("digits work in the name (a size, a model number)", async () => {
    const w = await openDialog();
    await typePrice("5");
    await click(next());
    await typeName("kids 40");
    await click(add());
    expect((w.emitted("add")![0] as any[])[1]).toBe("Kids 40");
  });

  it("Back returns to the price with it kept, and the name is kept too", async () => {
    await openDialog();
    await typePrice("30");
    await click(next());
    await typeName("hat");
    await click(q("[data-test=unknown-back]"));
    expect(q("[data-testid=unknown-price]")?.textContent?.trim()).toBe("$30");
    await click(next());
    expect(q("[data-testid=unknown-name]")?.textContent?.trim()).toBe("Hat");
  });

  it("stops at the name limit", async () => {
    await openDialog();
    await typePrice("5");
    await click(next());
    for (let i = 0; i < 90; i++) await click(button("a"));
    expect(q("[data-testid=unknown-name]")!.textContent!.trim().length).toBe(80);
  });

  it("opens afresh each time: back on the price, nothing typed", async () => {
    const w = await openDialog();
    await typePrice("30");
    await click(next());
    await typeName("hat");
    await w.setProps({ modelValue: false });
    await w.setProps({ modelValue: true });
    await flushPromises();
    expect(q("[data-testid=unknown-price]")?.textContent?.trim()).toBe("$0");
    await typePrice("1");
    await click(next());
    expect(q("[data-testid=unknown-name]")?.textContent?.trim()).toBe("Name");
  });

  it("a physical keyboard types the price, Enter goes on, then types the name, Enter adds", async () => {
    const w = await openDialog();
    for (const k of ["2", "0"]) await key(k);
    await key("Enter");
    expect(body().textContent).toContain("Name the item");
    for (const k of ["h", "a", "t"]) await key(k);
    await key("Enter");
    expect(w.emitted("add")).toEqual([[20, "hat"]]);
  });

  it("Enter on the price step with no price does not go on", async () => {
    await openDialog();
    await key("Enter");
    expect(body().textContent).not.toContain("Name the item");
  });

  it("the name step is a big one: a wide dialog, a tall name box and tall keys", async () => {
    const source = readFileSync(resolve(process.cwd(), "src/components/UnknownPriceDialog.vue"), "utf8");
    const num = (re: RegExp) => Number(source.match(re)![1]);
    // The dialog widens for the name step (the price step stays a narrow pad).
    expect(source).toMatch(/step === 'name' \? 880 : 360/);
    expect(num(/\.unknown-name-display \{[^}]*min-height: (\d+)px/)).toBeGreaterThanOrEqual(80);
    expect(num(/\.unknown-name-display \{[^}]*font-size: ([\d.]+)rem/)).toBeGreaterThanOrEqual(2.5);
    expect(num(/\.unknown-name-keyboard :deep\(\.inline-keyboard-key\) \{[^}]*height: (\d+)px/)).toBeGreaterThanOrEqual(72);
    expect(num(/\.unknown-name-keyboard :deep\(\.inline-keyboard-actions > \.k-btn\) \{[^}]*height: (\d+)px/)).toBeGreaterThanOrEqual(72);
    // And the keyboard is the one carrying that class.
    await openDialog();
    await typePrice("5");
    await click(next());
    expect(q(".unknown-name-keyboard")).toBeTruthy();
  });
});
