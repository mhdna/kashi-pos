// @vitest-environment happy-dom
//
// central's live-cart changes on the cart as the cashier sees them: an unknown
// item kashi set gets kashi's price and the "kashi" lock, the other unknown
// item stays as rung, and lifting the change gives the line its own back.
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

let wrapper: VueWrapper<any> | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
});

const unknown = (ref: string, price: number) => ({
  name: "Unknown item",
  code: "UNKNOWN",
  barcode: "UNKNOWN",
  price,
  originalPrice: price,
  qty: 1,
  discount: 0,
  unknownRef: ref,
});

async function cart() {
  wrapper = mount(InvoiceItems, { props: { unknownBarcode: "UNKNOWN" }, global: { plugins: [ui] } });
  await flushPromises();
  const vm = wrapper.vm as any;
  vm.addItem(unknown("u1", 30));
  vm.addItem(unknown("u2", 40));
  await flushPromises();
  return vm;
}

const rowsWithKashiLock = () =>
  wrapper!.findAll(".invoice-item-sale-option").filter((c) => c.text().includes("central")).length;

describe("InvoiceItems with central's live-cart changes", () => {
  it("puts central's price on the one unknown item it names", async () => {
    const vm = await cart();
    const changed = vm.applyRemoteOverrides([
      { barcode: "UNKNOWN", line_ref: "u2", price_cents: 5500, discount: 10, unknown_deal: "none" },
    ]);
    await flushPromises();
    expect(changed).toBe(true);
    const [u1, u2] = vm.items;
    expect([u1.price, u1.discount, u1.remote]).toEqual([30, 0, undefined]);
    expect([u2.price, u2.discount]).toEqual([55, 10]);
    expect(u2.remote).toMatchObject({ priceCents: 5500, discount: 10, unknownDeal: "none" });
    expect(rowsWithKashiLock()).toBe(1);
    // Sent again unchanged, nothing moves.
    expect(vm.applyRemoteOverrides([
      { barcode: "UNKNOWN", line_ref: "u2", price_cents: 5500, discount: 10, unknown_deal: "none" },
    ])).toBe(false);
  });

  it("gives the line its own price back when central lifts it", async () => {
    const vm = await cart();
    vm.applyRemoteOverrides([{ barcode: "UNKNOWN", line_ref: "u1", price_cents: 1000 }]);
    vm.applyRemoteOverrides([]);
    await flushPromises();
    expect(vm.items.map((i: any) => [i.price, i.remote])).toEqual([
      [30, undefined],
      [40, undefined],
    ]);
    expect(rowsWithKashiLock()).toBe(0);
  });

  it("doesn't give unknown items an override by barcode alone", async () => {
    const vm = await cart();
    expect(vm.applyRemoteOverrides([{ barcode: "UNKNOWN", price_cents: 100 }])).toBe(false);
  });
});
