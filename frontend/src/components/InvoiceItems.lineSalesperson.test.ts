// @vitest-environment happy-dom
//
// Who sold a line: the item details dialog lays the salespeople out as cards
// right in the dialog (no menu to open); picking a person (or "Invoice
// salesperson") sets that one line, and that is what checkout sends for it.
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

vi.mock("../../wailsjs/go/main/App", () => ({
  ListCategoryIcons: vi.fn().mockResolvedValue([]),
  ListSalespersons: vi.fn().mockResolvedValue([
    { id: 3, fullName: "Ali A", isSalesperson: true, isCurrent: true },
    { id: 5, fullName: "Fatima G", isSalesperson: true, isCurrent: true },
    { id: 8, fullName: "Door Guard", isSalesperson: false, isCurrent: true },
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
  document.body.innerHTML = "";
});

const shirt = (over: Record<string, unknown> = {}) => ({
  name: "Shirt", code: "SH1", barcode: "111", price: 50, originalPrice: 50, originalDiscount: 0, qty: 1, discount: 0, ...over,
});

async function openDetails(line = shirt()) {
  wrapper = mount(InvoiceItems, { global: { plugins: [ui] }, attachTo: document.body });
  await flushPromises();
  const vm = wrapper.vm as any;
  vm.addItem(line);
  await flushPromises();
  await wrapper.find(".invoice-item-details-trigger").trigger("click");
  await flushPromises();
  return vm;
}
const soldBy = () => document.body.querySelector<HTMLElement>("[data-test=line-salesperson]")!;
const tile = (id: string | number) => document.body.querySelector<HTMLElement>(`[data-test=sp-${id}]`);
const click = async (el: Element | null | undefined) => {
  el!.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  await flushPromises();
};

describe("item details: who sold this line", () => {
  it("says the line follows the invoice's salesperson until someone is picked", async () => {
    await openDetails();
    expect(soldBy().textContent).toContain("Invoice salesperson");
  });

  it("shows the cards in the dialog straight away, four to a row, scrolling", async () => {
    await openDetails();
    const box = document.body.querySelector<HTMLElement>(".line-salesperson-cards");
    expect(box).not.toBeNull();
    expect(box!.querySelector(".sp-grid--dense")).not.toBeNull();
    expect(box!.querySelector<HTMLElement>(".sp-grid")!.style.getPropertyValue("--sp-columns")).toBe("4");
    expect(document.body.querySelector(".line-salesperson-btn")).toBeNull();
    expect(tile(3)).not.toBeNull();
  });

  it("marks the line's current salesperson card as picked", async () => {
    await openDetails(shirt({ salespersonId: 5 }));
    const picked = document.body.querySelectorAll(".line-salesperson-cards [aria-selected=true]");
    expect(picked).toHaveLength(1);
    expect(picked[0]!.getAttribute("data-test")).toBe("sp-5");
  });

  it("offers the real salespeople as tiles, not attendance-only staff", async () => {
    await openDetails();
    expect(tile(3)?.textContent).toContain("Ali A");
    expect(tile(5)?.textContent).toContain("Fatima G");
    expect(tile(8)).toBeNull();
    expect(tile("default")?.textContent).toContain("Invoice salesperson");
  });

  it("picking a person sets that one line, names them and keeps the cards open", async () => {
    const vm = await openDetails();
    await click(tile(5));
    expect(vm.items[0].salespersonId).toBe(5);
    expect(soldBy().textContent).toContain("Fatima G");
    expect(tile(5)?.getAttribute("aria-selected")).toBe("true");
    expect(tile(3)?.getAttribute("aria-selected")).toBe("false");
    // Changing your mind is one more tap.
    await click(tile(3));
    expect(vm.items[0].salespersonId).toBe(3);
  });

  it("picking Invoice salesperson hands the line back", async () => {
    const vm = await openDetails(shirt({ salespersonId: 3 }));
    expect(soldBy().textContent).toContain("Ali A");
    await click(tile("default"));
    expect(vm.items[0].salespersonId).toBe(0);
    expect(soldBy().textContent).toContain("Invoice salesperson");
  });

  it("names a salesperson the roster no longer has rather than showing an id", async () => {
    await openDetails(shirt({ salespersonId: 77 }));
    expect(soldBy().textContent).toContain("Former salesperson");
  });
});
