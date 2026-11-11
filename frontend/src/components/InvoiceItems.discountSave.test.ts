// @vitest-environment happy-dom
//
// Checkout first saves each line's typed discount onto its product. Only a
// discount the cashier could type into the discount field is saved: one that
// came from central's live cart, from turning the discount off, or from
// editing the price must not be, or the till shows "custom item discounts are
// disabled" and refuses to check out.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

const { UpdateProductDiscount } = vi.hoisted(() => ({
  UpdateProductDiscount: vi.fn(),
}));

vi.mock("../../wailsjs/go/main/App", () => ({
  ListCategoryIcons: vi.fn().mockResolvedValue([]),
  ListSalespersons: vi.fn().mockResolvedValue([]),
  UpdateProductDiscount,
}));

vi.mock("@/lib/display", async () => {
  const { ref } = await import("vue");
  return { animationsEnabled: ref(false) };
});

import InvoiceItems from "./InvoiceItems.vue";

let wrapper: VueWrapper<any> | null = null;
beforeEach(() => {
  UpdateProductDiscount.mockReset();
  // As the backend answers with the custom-discount setting off.
  UpdateProductDiscount.mockRejectedValue(new Error("custom item discounts are disabled"));
});
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
});

const shirt = (discount = 10) => ({
  name: "Shirt",
  code: "SH1",
  barcode: "111",
  price: 50,
  originalPrice: 50,
  originalDiscount: discount,
  qty: 1,
  discount,
});

async function cart(props: Record<string, unknown>, lines = [shirt()]) {
  wrapper = mount(InvoiceItems, { props, global: { plugins: [ui] } });
  await flushPromises();
  const vm = wrapper.vm as any;
  for (const line of lines) vm.addItem(line);
  await flushPromises();
  return vm;
}

function errors(): string[] {
  return (wrapper!.emitted("discount-error") ?? []).map((e) => String(e[0]));
}

describe("saving discounts at checkout", () => {
  it("checks out a line whose discount came from a price edit, custom discounts off", async () => {
    const vm = await cart({ customPricesEnabled: true, customDiscountsEnabled: false });
    // What a price of 40 typed in discount mode does to the line.
    vm.items[0].discount = 20;
    vm.items[0].unitDiscounts = [20];
    expect(await vm.flushChanges()).toBe(true);
    expect(UpdateProductDiscount).not.toHaveBeenCalled();
    expect(errors()).toEqual([]);
    expect(vm.items[0].discount).toBe(20);
  });

  it("checks out a line with central's live-cart discount, custom discounts off", async () => {
    const vm = await cart({ customDiscountsEnabled: false });
    vm.applyRemoteOverrides([{ barcode: "111", discount: 35 }]);
    expect(vm.items[0].discount).toBe(35);
    expect(await vm.flushChanges()).toBe(true);
    expect(UpdateProductDiscount).not.toHaveBeenCalled();
    expect(errors()).toEqual([]);
    expect(vm.items[0].discount).toBe(35);
  });

  it("never saves central's live-cart discount onto the product, custom discounts on", async () => {
    UpdateProductDiscount.mockResolvedValue(undefined);
    const vm = await cart({ customDiscountsEnabled: true });
    vm.applyRemoteOverrides([{ barcode: "111", discount: 35 }]);
    expect(await vm.flushChanges()).toBe(true);
    expect(UpdateProductDiscount).not.toHaveBeenCalled();
  });

  it("never saves a discount turned off as the product's discount", async () => {
    UpdateProductDiscount.mockResolvedValue(undefined);
    const vm = await cart({ customDiscountsEnabled: true, discountToggleEnabled: true });
    vm.items[0].discountOff = true;
    vm.items[0].discount = 0;
    vm.items[0].unitDiscounts = [0];
    expect(await vm.flushChanges()).toBe(true);
    expect(UpdateProductDiscount).not.toHaveBeenCalled();
  });

  it("checks out an untouched cart of many lines without saving anything", async () => {
    const vm = await cart({ customDiscountsEnabled: false }, [
      shirt(0),
      { ...shirt(15), code: "SH2", barcode: "222" },
      { ...shirt(5), code: "SH3", barcode: "333" },
    ]);
    expect(await vm.flushChanges()).toBe(true);
    expect(UpdateProductDiscount).not.toHaveBeenCalled();
  });

  it("still saves a discount the cashier typed, custom discounts on", async () => {
    UpdateProductDiscount.mockResolvedValue(undefined);
    const vm = await cart({ customDiscountsEnabled: true });
    vm.items[0].discount = 25;
    expect(await vm.flushChanges()).toBe(true);
    expect(UpdateProductDiscount).toHaveBeenCalledWith("SH1", 25);
    expect(wrapper!.emitted("discount-saved")?.[0]).toEqual(["SH1", 25]);
  });

  it("does not save a typed discount on a code the branch keeps locked", async () => {
    UpdateProductDiscount.mockResolvedValue(undefined);
    const vm = await cart({
      customDiscountsEnabled: true,
      priceEditRestricted: true,
      priceEditCodes: ["other"],
    });
    vm.items[0].discount = 25;
    expect(await vm.flushChanges()).toBe(true);
    expect(UpdateProductDiscount).not.toHaveBeenCalled();
  });
});
