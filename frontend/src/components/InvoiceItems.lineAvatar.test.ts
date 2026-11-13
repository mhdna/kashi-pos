// @vitest-environment happy-dom
//
// A cart line set to its own salesperson shows their avatar before the name.
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

vi.mock("../../wailsjs/go/main/App", () => ({
  ListCategoryIcons: vi.fn().mockResolvedValue([]),
  ListSalespersons: vi.fn().mockResolvedValue([
    { id: 3, fullName: "Ali Abbas", isSalesperson: true, isCurrent: true },
    { id: 5, fullName: "Fatima Gh", isSalesperson: true, isCurrent: true },
  ]),
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

const line = (over: Record<string, unknown> = {}) => ({
  name: "Shirt", code: "SH1", barcode: "111", price: 50, originalPrice: 50, originalDiscount: 0, qty: 1, discount: 0, ...over,
});
async function cart(lines: Record<string, unknown>[]) {
  wrapper = mount(InvoiceItems, { global: { plugins: [ui] } });
  await flushPromises();
  for (const l of lines) (wrapper.vm as any).addItem(l);
  await flushPromises();
  return wrapper;
}
const seller = (w: VueWrapper<any>, i = 0) => w.findAll(".invoice-item-name-row")[i]!.find("[data-test=line-seller]");

describe("line salesperson avatar", () => {
  it("shows no avatar for a line that follows the invoice's salesperson", async () => {
    const w = await cart([line(), line({ code: "SH2", barcode: "222", salespersonId: 0 })]);
    expect(w.find("[data-test=line-seller]").exists()).toBe(false);
  });

  it("shows their initials before the name, with who it is in the tooltip", async () => {
    const w = await cart([line({ salespersonId: 5 })]);
    const row = w.find(".invoice-item-name-row");
    const avatar = seller(w);
    expect(avatar.exists()).toBe(true);
    expect(avatar.text()).toBe("FG");
    expect(avatar.attributes("title")).toBe("Sold by Fatima Gh");
    // Before the name, in the same row.
    const kids = Array.from(row.element.children);
    expect(kids.indexOf(avatar.element)).toBeLessThan(kids.indexOf(row.find(".invoice-item-name").element));
    expect(row.find(".invoice-item-name").text()).toBe("Shirt");
  });

  it("gives each line its own person's avatar", async () => {
    const w = await cart([line({ salespersonId: 3 }), line({ code: "SH2", barcode: "222", name: "Jeans", salespersonId: 5 }), line({ code: "SH3", barcode: "333", name: "Hat" })]);
    const byName: Record<string, string | false> = {};
    for (const r of w.findAll(".invoice-item-name-row")) {
      const s = r.find("[data-test=line-seller]");
      byName[r.find(".invoice-item-name").text()] = s.exists() && s.text();
    }
    expect(byName).toEqual({ Shirt: "AA", Jeans: "FG", Hat: false });
  });

  it("names a salesperson who is no longer on the roster rather than hiding them", async () => {
    const w = await cart([line({ salespersonId: 77 })]);
    expect(seller(w).attributes("title")).toBe("Sold by Former salesperson");
  });

  it("follows the line when it is changed or cleared", async () => {
    const w = await cart([line()]);
    expect(seller(w).exists()).toBe(false);
    (w.vm as any).items[0].salespersonId = 3;
    await flushPromises();
    expect(seller(w).text()).toBe("AA");
    (w.vm as any).items[0].salespersonId = 0;
    await flushPromises();
    expect(seller(w).exists()).toBe(false);
  });
});
