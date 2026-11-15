// @vitest-environment happy-dom
//
// The cart's column header (Item / Qty / Disc. / Price) is switched off for
// now, so each line's quantity says "Qty" again.
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
describe("cart lines without a column header", () => {
  it("shows no column header", async () => {
    const w = await cart({ customDiscountsEnabled: true }, [shirt()]);
    expect(w.find("[data-test=invoice-items-header]").exists()).toBe(false);
  });

  it("each line's quantity says Qty before the number", async () => {
    const w = await cart({}, [shirt({ qty: 3 })]);
    const qty = w.find(".invoice-qty-control");
    expect(qty.find(".number-pad-affix").exists()).toBe(true);
    expect(qty.text().replace(/\s+/g, " ")).toMatch(/Qty\s*3/);
  });

  it("is one block per line, with nothing above them", async () => {
    const w = await cart({}, [shirt(), shirt({ code: "SH2", barcode: "222" }), shirt({ code: "SH3", barcode: "333" })]);
    const blocks = Array.from(w.element.children as HTMLCollection).filter((el): el is HTMLElement => el.tagName === "DIV");
    expect(blocks).toHaveLength(3);
    expect(blocks.every((b) => b.querySelector(".invoice-item-actions"))).toBe(true);
  });
});
