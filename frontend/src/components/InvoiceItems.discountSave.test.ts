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
  // The cases below are for a branch that remembers discounts unless they say.
  wrapper = mount(InvoiceItems, { props: { rememberDiscounts: true, ...props }, global: { plugins: [ui] } });
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

describe("not remembering typed discounts (the default)", () => {
  // As the discount field does: the line and each of its units.
  function type(line: any, discount: number) {
    line.discount = discount;
    line.unitDiscounts = Array(line.qty).fill(discount);
  }
  const off = (extra: Record<string, unknown> = {}) =>
    cart({ rememberDiscounts: false, customDiscountsEnabled: true, ...extra });

  it("never saves a typed discount onto the product", async () => {
    const vm = await off();
    type(vm.items[0], 25);
    expect(await vm.flushChanges()).toBe(true);
    expect(UpdateProductDiscount).not.toHaveBeenCalled();
    expect(errors()).toEqual([]);
    expect(vm.items[0].discount).toBe(25);
  });

  const pieces = (vm: any) => vm.items.map((i: any) => [i.qty, i.unitDiscounts]);

  it("is off when the setting is never passed", async () => {
    wrapper = mount(InvoiceItems, { props: { customDiscountsEnabled: true }, global: { plugins: [ui] } });
    const vm = wrapper.vm as any;
    vm.addItem(shirt());
    type(vm.items[0], 25);
    vm.addItem(shirt());
    await vm.flushChanges();
    expect(UpdateProductDiscount).not.toHaveBeenCalled();
    expect(pieces(vm)).toEqual([[2, [25, 10]]]);
  });

  it("gives each next piece scanned the list discount, on the same line", async () => {
    const vm = await off();
    type(vm.items[0], 25);
    vm.addItem(shirt());
    vm.addItem(shirt());
    expect(pieces(vm)).toEqual([[3, [25, 10, 10]]]);
    expect(vm.discountedSubtotalCents).toBe(3750 + 4500 + 4500);
  });

  it("does the same for a discount typed on one piece only, or typed as 0", async () => {
    const vm = await off();
    vm.addItem(shirt());
    vm.items[0].unitDiscounts = [10, 30];
    vm.addItem(shirt());
    expect(pieces(vm)).toEqual([[3, [10, 30, 10]]]);
    type(vm.items[0], 0);
    vm.addItem(shirt());
    expect(pieces(vm)).toEqual([[4, [0, 0, 0, 10]]]);
  });

  it("shows the pieces' discounts on the line only when they differ", async () => {
    const vm = await off();
    vm.addItem(shirt());
    await flushPromises();
    const note = () => wrapper!.find('[data-test="mixed-discounts"]');
    expect(note().exists()).toBe(false);
    type(vm.items[0], 25);
    vm.addItem(shirt());
    await flushPromises();
    expect(note().text()).toContain("25%, 25%, 10%");
  });

  it("leaves a turned-off discount and central's discount to every piece", async () => {
    const vm = await off({ discountToggleEnabled: true });
    vm.items[0].discountOff = true;
    type(vm.items[0], 0);
    vm.addItem(shirt());
    expect(pieces(vm)).toEqual([[2, [0, 0]]]);
    vm.items[0].discountOff = false;
    type(vm.items[0], 10);
    vm.applyRemoteOverrides([{ barcode: "111", discount: 35 }]);
    vm.addItem(shirt());
    expect(pieces(vm)).toEqual([[3, [35, 35, 35]]]);
  });

  it("carries a typed discount to the next piece when the branch remembers", async () => {
    const vm = await cart({ customDiscountsEnabled: true });
    type(vm.items[0], 25);
    vm.addItem(shirt());
    expect(pieces(vm)).toEqual([[2, [25, 25]]]);
  });

  it("randomized: every scanned piece keeps the discount it was given", async () => {
    let seed = 7;
    const random = (n: number) => (seed = (seed * 48271) % 2147483647) % n;
    for (let trial = 0; trial < 200; trial++) {
      const vm = await off();
      let want = [10];
      for (let step = 0; step < 12; step++) {
        if (random(3) === 0) {
          const discount = random(101);
          type(vm.items[0], discount);
          want = want.map(() => discount);
        } else {
          vm.addItem(shirt());
          want.push(10);
        }
        expect(pieces(vm)).toEqual([[want.length, want]]);
        expect(vm.discountedSubtotalCents).toBe(want.reduce((sum, d) => sum + 5000 - 50 * d, 0));
      }
      await vm.flushChanges();
      wrapper!.unmount();
      wrapper = null;
    }
    expect(UpdateProductDiscount).not.toHaveBeenCalled();
  });
});
