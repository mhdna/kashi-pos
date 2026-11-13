// @vitest-environment happy-dom
//
// The cart's column header: Item / Qty / Disc. / Price once, above the lines,
// so the lines carry just the numbers (no "Qty" word on each).
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
});

const shirt = (over: Record<string, unknown> = {}) => ({
  name: "Shirt", code: "SH1", barcode: "111", price: 50, originalPrice: 50, originalDiscount: 0, qty: 1, discount: 0, ...over,
});

async function cart(props: Record<string, unknown>, lines: Record<string, unknown>[]) {
  wrapper = mount(InvoiceItems, { props, global: { plugins: [ui] } });
  await flushPromises();
  for (const line of lines) (wrapper.vm as any).addItem(line);
  await flushPromises();
  return wrapper;
}
const titles = (w: VueWrapper<any>) =>
  w.findAll(".invoice-items-header > span:not(.invoice-header-delete)").map((s) => s.text());

describe("cart column header", () => {
  it("is not there for an empty cart", async () => {
    const w = await cart({}, []);
    expect(w.find("[data-test=invoice-items-header]").exists()).toBe(false);
  });

  it("names the columns once above the lines", async () => {
    const w = await cart({}, [shirt(), shirt({ code: "SH2", barcode: "222" })]);
    expect(w.findAll("[data-test=invoice-items-header]")).toHaveLength(1);
    expect(titles(w)).toEqual(["Item", "Qty", "Price"]);
  });

  it("adds a Disc. column only when a line's discount can be edited", async () => {
    const w = await cart({ customDiscountsEnabled: true }, [shirt()]);
    expect(titles(w)).toEqual(["Item", "Qty", "Disc.", "Price"]);
    // And the lines have the matching discount cell.
    expect(w.findAll(".invoice-discount-control")).toHaveLength(1);
  });

  it("keeps the header columns as wide as the line controls (qty, discount, price, delete)", async () => {
    const w = await cart({ customDiscountsEnabled: true }, [shirt()]);
    const header = w.find(".invoice-items-header");
    // One header cell per control in the line, in the same order.
    const line = w.find(".invoice-item-actions");
    expect(line.findAll(".invoice-qty-control, .invoice-discount-control, .invoice-price-control, .invoice-delete-control")).toHaveLength(4);
    expect(header.findAll(".invoice-header-qty, .invoice-header-discount, .invoice-header-price, .invoice-header-delete")).toHaveLength(4);
  });

  it("no line carries the word Qty any more: the number stands alone", async () => {
    const w = await cart({}, [shirt({ qty: 3 })]);
    const qty = w.find(".invoice-qty-control");
    expect(qty.text()).toBe("3");
    expect(qty.find(".number-pad-affix").exists()).toBe(false);
    // The word lives only in the header.
    const rowText = w.findAll(".invoice-item-actions").map((a) => a.text()).join(" ");
    expect(rowText).not.toContain("Qty");
  });

  it("is a table of direct children: the header first, then one block per line (the borders rely on it)", async () => {
    const w = await cart({}, [shirt(), shirt({ code: "SH2", barcode: "222" }), shirt({ code: "SH3", barcode: "333" })]);
    const blocks = Array.from(w.element.children as HTMLCollection).filter((el): el is HTMLElement => el.tagName === "DIV");
    expect(blocks[0]!.classList.contains("invoice-items-header")).toBe(true);
    expect(blocks).toHaveLength(4); // header + 3 lines
    expect(blocks.slice(1).every((b) => b.querySelector(".invoice-item-actions"))).toBe(true);
  });
});
