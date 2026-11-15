// @vitest-environment happy-dom
//
// Who sold a line: the item details dialog has a plain dropdown of names (no
// avatars, no cards); picking a person (or "Invoice salesperson") sets that one
// line, and that is what checkout sends for it.
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
const select = () => wrapper!.findComponent({ name: "KSelect" });
const titles = () => (select().props("items") as { title: string; value: number }[]).map((o) => o.title);
const pick = async (value: number) => {
  select().vm.$emit("update:modelValue", value);
  await flushPromises();
};

describe("item details: who sold this line", () => {
  it("says the line follows the invoice's salesperson until someone is picked", async () => {
    await openDetails();
    expect(soldBy().textContent).toContain("Sold by");
    expect(select().props("modelValue")).toBe(0);
    expect(soldBy().textContent).toContain("Invoice salesperson");
  });

  it("is a dropdown of names: no cards, no avatars", async () => {
    await openDetails();
    expect(select().exists()).toBe(true);
    expect(document.body.querySelector(".line-salesperson-cards")).toBeNull();
    expect(soldBy().querySelector(".k-avatar, [data-test^=sp-]")).toBeNull();
  });

  it("lists Invoice salesperson first, then the real salespeople, not attendance-only staff", async () => {
    await openDetails();
    expect(titles()).toEqual(["Invoice salesperson", "Ali A", "Fatima G"]);
  });

  it("shows the line's current salesperson as picked", async () => {
    await openDetails(shirt({ salespersonId: 5 }));
    expect(select().props("modelValue")).toBe(5);
    expect(soldBy().textContent).toContain("Fatima G");
  });

  it("picking a person sets that one line, and changing your mind is one more pick", async () => {
    const vm = await openDetails();
    await pick(5);
    expect(vm.items[0].salespersonId).toBe(5);
    await pick(3);
    expect(vm.items[0].salespersonId).toBe(3);
  });

  it("picking Invoice salesperson hands the line back", async () => {
    const vm = await openDetails(shirt({ salespersonId: 3 }));
    await pick(0);
    expect(vm.items[0].salespersonId).toBe(0);
  });

  it("names a salesperson the roster no longer has rather than showing an id", async () => {
    await openDetails(shirt({ salespersonId: 77 }));
    expect(titles()).toContain("Former salesperson");
    expect(soldBy().textContent).toContain("Former salesperson");
  });
});
