// @vitest-environment happy-dom
//
// An unknown item's price can be retyped from the price cell's pad, even when
// the branch doesn't allow custom prices; any other line still follows that
// setting.
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

const unknown = (over: Record<string, unknown> = {}) => ({
  name: "Red scarf", code: "UNKNOWN", barcode: "UNKNOWN", unknownRef: "r1", price: 25, originalPrice: 25, originalDiscount: 0, qty: 1, discount: 0, ...over,
});
const shirt = () => ({ name: "Shirt", code: "SH1", barcode: "111", price: 50, originalPrice: 50, originalDiscount: 0, qty: 1, discount: 0 });

async function cart(props: Record<string, unknown>, lines: Record<string, unknown>[]) {
  wrapper = mount(InvoiceItems, { props, global: { plugins: [ui] }, attachTo: document.body });
  await flushPromises();
  for (const l of lines) (wrapper.vm as any).addItem(l);
  await flushPromises();
  return wrapper;
}
const click = async (el: Element | null | undefined) => {
  el!.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  await flushPromises();
};
const priceCell = (w: VueWrapper<any>, index = 0) => w.findAll(".invoice-price-control")[index]!;
const pad = (label: string) =>
  Array.from(document.body.querySelectorAll<HTMLElement>(".number-pad button")).find((b) => b.textContent?.trim() === label) ?? null;
const retype = async (w: VueWrapper<any>, digits: string) => {
  await click(w.find(".invoice-price-editor").element);
  for (const d of digits) await click(pad(d));
  await click(pad("Done"));
};

describe("unknown item price", () => {
  it("is a button that opens the Price pad, with custom prices off", async () => {
    const w = await cart({ customPricesEnabled: false }, [unknown()]);
    expect(priceCell(w).element.tagName).toBe("BUTTON");
    await click(priceCell(w).element);
    expect(document.body.querySelector(".number-pad-title")?.textContent).toBe("Price");
  });

  it("a retyped price becomes the price: the line, its units and what checkout sends", async () => {
    const w = await cart({ customPricesEnabled: false, priceChangeMode: "discount" }, [unknown({ qty: 2 })]);
    await retype(w, "40");
    const line = (w.vm as any).items[0];
    expect(line.price).toBe(40);
    expect(line.originalPrice).toBe(40); // no shelf price to discount from
    expect(line.discount).toBe(0); // and no discount made out of the change
    expect(line.unitPrices).toEqual([40, 40]);
    expect(priceCell(w).text()).toContain("$40");
  });

  it("the other price mode retypes it the same way", async () => {
    const w = await cart({ customPricesEnabled: false, priceChangeMode: "override" }, [unknown()]);
    await retype(w, "7.5");
    const line = (w.vm as any).items[0];
    expect(line.price).toBe(7.5);
    expect(line.unitPrices).toEqual([7.5]);
  });

  it("keeps a discount typed on the line, applying it to the new price", async () => {
    const w = await cart({ customPricesEnabled: false }, [unknown({ discount: 10 })]);
    await retype(w, "100");
    const line = (w.vm as any).items[0];
    expect(line.price).toBe(100);
    expect(line.discount).toBe(10);
    expect(line.unitDiscounts).toEqual([10]);
  });

  it("starts the pad from the price it has now", async () => {
    const w = await cart({}, [unknown({ price: 25 })]);
    await click(w.find(".invoice-price-editor").element);
    expect(document.body.querySelector(".number-pad-display")?.textContent).toContain("25");
  });

  it("another line is still locked while custom prices are off, and opens them when on", async () => {
    const off = await cart({ customPricesEnabled: false }, [shirt()]);
    expect(off.find(".invoice-price-editor").exists()).toBe(false);
    off.unmount();
    wrapper = null;
    const on = await cart({ customPricesEnabled: true }, [shirt()]);
    expect(on.find(".invoice-price-editor").exists()).toBe(true);
  });

  it("a locked or read-only unknown line cannot be retyped", async () => {
    const w = await cart({ customPricesEnabled: false, readonly: true }, [unknown()]);
    expect(w.find(".invoice-price-editor").exists()).toBe(false);
  });
});
