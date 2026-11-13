// @vitest-environment happy-dom
//
// A held sale put back on the cart, as the cashier sees it: the custom
// prices, the custom discount and central's price are all still there, and
// central's overrides coming back for the same sale (ResumeHoldInvoice) leave
// it as held. The old restore handed the sale a new ref, so central's
// overrides came back empty and its line fell back to the list price.
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

vi.mock("../../wailsjs/go/main/App", () => ({
  ListCategoryIcons: vi.fn().mockResolvedValue([]),
  ListSalespersons: vi.fn().mockResolvedValue([]),
  UpdateProductDiscount: vi.fn().mockResolvedValue(undefined),
}));

// Node's own (empty) localStorage shadows happy-dom's here.
vi.mock("@/lib/display", async () => {
  const { ref } = await import("vue");
  return { animationsEnabled: ref(false) };
});

import InvoiceItems from "./InvoiceItems.vue";
import { buildHoldSnapshot, heldCart } from "@/lib/holdSnapshot";
import heldSale from "@/lib/testdata/held_sale.json";

let wrapper: VueWrapper<any> | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
});

async function cart() {
  wrapper = mount(InvoiceItems, {
    props: { unknownBarcode: "UNKNOWN", customPricesEnabled: true, customDiscountsEnabled: true, perUnitPricesEnabled: true },
    global: { plugins: [ui] },
  });
  await flushPromises();
  return wrapper.vm as any;
}

// central's overrides of the sample sale (hold_resume_test.go).
const centralJacket = [{ barcode: "bc-JACKET", price_cents: 9000, discount: 50 }];

function rowTexts() {
  return wrapper!.findAll(".invoice-price-value").map((w) => w.text());
}

describe("a held sale back on the cart", () => {
  it("shows every line as held, central's line still marked central's", async () => {
    const vm = await cart();
    vm.items = heldCart(heldSale, () => "fresh").restore.items;
    await flushPromises();
    expect(vm.items).toEqual(heldSale.items);
    expect(vm.items.map((i: any) => [i.code, i.price, i.discount])).toEqual([
      ["A", 85, 0],
      ["B", 80, 15],
      ["JACKET", 90, 50],
      ["SOCKS", 20, 0],
      ["UNKNOWN", 12.5, 0],
    ]);
    expect(rowTexts().join(" ")).toContain("85");
    expect(wrapper!.findAll(".invoice-item-sale-option").filter((c) => c.text().includes("central")).length).toBe(1);
  });

  it("stays as held when central's overrides of the same sale come back", async () => {
    const vm = await cart();
    vm.items = heldCart(heldSale, () => "fresh").restore.items;
    await flushPromises();
    expect(vm.applyRemoteOverrides(centralJacket)).toBe(false);
    expect(vm.items).toEqual(heldSale.items);
  });

  it("would lose central's price without them (what a new sale ref did)", async () => {
    const vm = await cart();
    vm.items = heldCart(heldSale, () => "fresh").restore.items;
    await flushPromises();
    expect(vm.applyRemoteOverrides([])).toBe(true);
    expect(vm.items[2]).toMatchObject({ price: 100, discount: 0 });
  });

  it("holds what the cashier rang up and puts it back the same", async () => {
    const vm = await cart();
    vm.addItem({ name: "Item A", code: "A", barcode: "bc-A", price: 100, discount: 0, qty: 1 });
    vm.addItem({ name: "Item A", code: "A", barcode: "bc-A", price: 100, discount: 0, qty: 1 });
    vm.addItem({ name: "Unknown item", code: "UNKNOWN", barcode: "UNKNOWN", price: 30, qty: 1, originalPrice: 30, unknownRef: "u9" });
    vm.items[0].price = 85;
    vm.items[0].unitPrices = [85, 90];
    vm.items[0].unitDiscounts = [10, 0];
    vm.applyRemoteOverrides([{ barcode: "UNKNOWN", line_ref: "u9", price_cents: 2500 }]);
    await flushPromises();
    const rung = JSON.parse(JSON.stringify(vm.items));
    const hold = JSON.parse(JSON.stringify(buildHoldSnapshot({ items: vm.items, saleRef: "s9", unknownClientRef: "p9" })));
    vm.clearItems();
    await flushPromises();
    const held = heldCart({ ...hold, id: 1 }, () => "fresh");
    vm.items = held.restore.items;
    await flushPromises();
    expect(vm.items).toEqual(rung);
    expect([held.saleRef, held.unknownClientRef]).toEqual(["s9", "p9"]);
    expect(vm.applyRemoteOverrides([{ barcode: "UNKNOWN", line_ref: "u9", price_cents: 2500 }])).toBe(false);
  });
});
