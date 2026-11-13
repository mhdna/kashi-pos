// @vitest-environment happy-dom
//
// The number pad says what it is changing, above the number: "Quantity",
// "Discount" or "Price" (and, in the item details, which unit). No item name.
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

async function cart(props: Record<string, unknown>, lines = [shirt()]) {
  wrapper = mount(InvoiceItems, { props, global: { plugins: [ui] }, attachTo: document.body });
  await flushPromises();
  for (const l of lines) (wrapper.vm as any).addItem(l);
  await flushPromises();
  return wrapper;
}
const open = async (el: Element | null) => {
  el!.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  await flushPromises();
};
const heading = () => document.body.querySelector("[data-test=number-pad-heading]");
const title = () => heading()?.querySelector(".number-pad-title")?.textContent;

describe("number pad heading", () => {
  it("names the quantity", async () => {
    const w = await cart({});
    await open(w.find(".invoice-qty-control .number-pad-activator").element);
    expect(title()).toBe("Quantity");
    expect(heading()!.textContent!.trim()).toBe("Quantity"); // no item name
  });

  it("names the discount", async () => {
    const w = await cart({ customDiscountsEnabled: true });
    await open(w.find(".invoice-discount-control .number-pad-activator").element);
    expect(title()).toBe("Discount");
    expect(heading()!.textContent!.trim()).toBe("Discount");
  });

  it("names the price", async () => {
    const w = await cart({ customPricesEnabled: true });
    await open(w.find(".invoice-price-editor").element);
    expect(title()).toBe("Price");
    expect(heading()!.textContent!.trim()).toBe("Price");
  });

  it("a pad without a title shows no heading (the close-shift pads are unchanged)", async () => {
    const { default: NumberPadMenu } = await import("./NumberPadMenu.vue");
    const w = mount(NumberPadMenu, { props: { modelValue: 5 }, global: { plugins: [ui] }, attachTo: document.body });
    await open(w.find(".number-pad-activator").element);
    expect(heading()).toBeNull();
    w.unmount();
  });

  it("the C (clear) key is the red one; backspace is a soft amber key", async () => {
    const { default: NumberPadMenu } = await import("./NumberPadMenu.vue");
    const w = mount(NumberPadMenu, { props: { modelValue: 5 }, global: { plugins: [ui] }, attachTo: document.body });
    await open(w.find(".number-pad-activator").element);
    const clear = document.body.querySelector("[data-test=number-pad-clear]")!;
    const back = document.body.querySelector("[data-test=number-pad-backspace]")!;
    expect(clear.textContent!.trim()).toBe("C");
    expect(clear.className).toContain("k-btn--variant-flat");
    expect(clear.getAttribute("style") ?? "").toMatch(/error/);
    expect(back.className).toContain("k-btn--variant-tonal");
    expect(back.getAttribute("style") ?? "").toMatch(/warning/);
    expect(back.getAttribute("style") ?? "").not.toMatch(/error/);
    expect(back.querySelector(".k-icon")).toBeTruthy();
    w.unmount();
  });
});
