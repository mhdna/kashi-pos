// @vitest-environment happy-dom
//
// "$50 off every $100, instead of item discounts": when the quote says a
// line went without its 20%, the cart shows it at full price with the chip
// struck, so the rows add up to what the deal comes off.
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";
import golden from "@/lib/testdata/replaced_discount_quotes.json";

vi.mock("../../wailsjs/go/main/App", () => ({
  ListCategoryIcons: vi.fn().mockResolvedValue([]),
  ListSalespersons: vi.fn().mockResolvedValue([]),
}));
vi.mock("@/lib/display", async () => {
  const { ref } = await import("vue");
  return { animationsEnabled: ref(false) };
});

import InvoiceItems from "./InvoiceItems.vue";
import type { DealLine } from "@/lib/deals";

let wrapper: VueWrapper<any> | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  document.body.innerHTML = "";
});

const jacket = { name: "Jacket", code: "JACKET", barcode: "bc-JACKET", price: 100, originalPrice: 100, originalDiscount: 20, qty: 1, discount: 20 };
const socks = { name: "Socks", code: "SOCKS", barcode: "bc-SOCKS", price: 20, originalPrice: 20, originalDiscount: 20, qty: 1, discount: 20 };

async function cart(dealLines: unknown) {
  wrapper = mount(InvoiceItems, { props: { dealLines: dealLines as DealLine[] }, global: { plugins: [ui] }, attachTo: document.body });
  await flushPromises();
  for (const l of [jacket, socks]) (wrapper.vm as any).addItem({ ...l });
  await flushPromises();
  return wrapper;
}
const amounts = (w: VueWrapper<any>) =>
  w.findAll(".invoice-price-values .font-weight-bold").map((el) => el.text());
const chips = (w: VueWrapper<any>) => w.findAll(".invoice-price-discount .text-decoration-line-through");

describe("a line whose discount the invoice deal replaced", () => {
  it("shows at full price with its 20% struck", async () => {
    const w = await cart(golden.replaced.lines);
    const shown = amounts(w).join(" ");
    expect(shown).toContain("100");
    expect(shown).not.toContain("80");
    expect(chips(w)).toHaveLength(2);
    expect(chips(w)[0]!.text()).toBe("-20%");
  });

  it("keeps its 20% when the deal didn't apply", async () => {
    const w = await cart(golden.kept.lines);
    const shown = amounts(w).join(" ");
    expect(shown).toContain("80");
    expect(shown).toContain("16");
    expect(chips(w)).toHaveLength(0);
  });
});
