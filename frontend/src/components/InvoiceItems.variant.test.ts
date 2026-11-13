// @vitest-environment happy-dom
//
// A cart line's colour is a dot of that colour beside its name, and its size a
// small round pill, instead of one grey "Green / Tra" string.
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

const line = (over: Record<string, unknown> = {}) => ({
  name: "Jacket", code: "JK1", barcode: "111", price: 50, originalPrice: 50, originalDiscount: 0, qty: 1, discount: 0, ...over,
});

async function cart(lines: Record<string, unknown>[]) {
  wrapper = mount(InvoiceItems, { global: { plugins: [ui] } });
  await flushPromises();
  for (const l of lines) (wrapper.vm as any).addItem(l);
  await flushPromises();
  return wrapper;
}

describe("cart line colour and size", () => {
  it("shows the colour as a dot of that colour next to its name", async () => {
    const w = await cart([line({ color: "Green" })]);
    const color = w.find("[data-test=line-color]");
    expect(color.text()).toBe("Green");
    const dot = color.find(".color-swatch");
    expect(dot.exists()).toBe(true);
    expect(dot.attributes("data-stops")).toMatch(/#/); // painted, not the empty dashed dot
    expect(w.find("[data-test=line-size]").exists()).toBe(false);
  });

  it("shows a two-colour name as one dot sliced in both colours", async () => {
    const w = await cart([line({ color: "Red/Blue" })]);
    expect(w.find("[data-test=line-color] .color-swatch").attributes("data-stops")!.split(" ")).toHaveLength(2);
  });

  it("shows the size as its own pill, separate from the colour", async () => {
    const w = await cart([line({ color: "Green", size: "XL" })]);
    expect(w.find("[data-test=line-color]").text()).toBe("Green");
    expect(w.find("[data-test=line-size]").text()).toBe("XL");
    expect(w.text()).not.toContain("Green / XL");
  });

  it("shows a size alone with no dot", async () => {
    const w = await cart([line({ size: "M" })]);
    expect(w.find("[data-test=line-size]").text()).toBe("M");
    expect(w.find("[data-test=line-color]").exists()).toBe(false);
  });

  it("shows neither for a line with no colour or size", async () => {
    const w = await cart([line()]);
    expect(w.find(".invoice-item-variant").exists()).toBe(false);
  });

  it("an abbreviated name like Ora/Cuv still gets its dot (unknown parts are skipped)", async () => {
    const w = await cart([line({ color: "Ora/Cuv" })]);
    expect(w.find("[data-test=line-color]").text()).toBe("Ora/Cuv");
    expect(w.find("[data-test=line-color] .color-swatch").exists()).toBe(true);
  });

  it("a named unknown item shows its name, and is still the unknown placeholder underneath", async () => {
    const w = await cart([line({ name: "Red scarf", code: "UNKNOWN", barcode: "UNKNOWN", unknownRef: "r1" })]);
    expect(w.find(".invoice-item-name").text()).toBe("Red scarf");
    // Still handled as an unknown line: the picture-taking tint, no price edit.
    expect(w.find(".invoice-item-unknown").exists()).toBe(true);
    // Its checkout line carries the name the cashier gave it.
    const sent = JSON.parse(JSON.stringify((w.vm as any).items))[0];
    expect(sent.name).toBe("Red scarf");
    expect(sent.barcode).toBe("UNKNOWN");
  });
});
