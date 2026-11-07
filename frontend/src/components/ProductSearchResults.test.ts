// @vitest-environment happy-dom
//
// The product search results: variants sharing a code show as one result
// with their colours and sizes, and tapping it opens the variant picker.
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

vi.mock("../../wailsjs/go/main/App", () => ({
  ListProductsCursor: vi.fn(),
  CountProductGroupsForSearch: vi.fn(),
  ListProductVariants: vi.fn(),
}));

import { CountProductGroupsForSearch, ListProductsCursor, ListProductVariants } from "../../wailsjs/go/main/App";
import ProductSearchResults from "./ProductSearchResults.vue";

const listCursor = vi.mocked(ListProductsCursor);
const countGroups = vi.mocked(CountProductGroupsForSearch);
const listVariants = vi.mocked(ListProductVariants);

beforeAll(() => {
  if (!("ResizeObserver" in globalThis)) {
    (globalThis as any).ResizeObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    };
  }
});

type Row = Record<string, any>;
let id = 0;
const row = (code: string, color: string, size: string, price = 29, name = "Kids Casual shoes"): Row => {
  id++;
  return { id, code, name, barcode: `${code}-${id}`, category: "", price, discount: 0, color, size };
};
// The screenshot: 8 + 4 variants of two codes, and an ordinary product.
function catalogue(): Row[] {
  id = 13405;
  const out: Row[] = [];
  for (const size of ["20-25", "25-30"]) for (const color of ["Grey", "Beige", "White", "Black"]) out.push(row("C6KSHE29-LC", color, size));
  for (const color of ["Grey", "Beige", "White", "Black"]) out.push(row("C6KSHE35-LC", color, "31-36", 35));
  out.push(row("SCARF-1", "Red", "", 12, "Wool scarf"));
  return out;
}

// Pages of `size` rows from `rows`, like ListProductsCursor.
function serve(rows: Row[], size = 50) {
  listCursor.mockImplementation(async (after: number) => {
    const rest = rows.filter((r) => r.id > after);
    const items = rest.slice(0, size);
    return { items, nextCursor: items.at(-1)?.id ?? after, hasMore: rest.length > size, total: -1 } as any;
  });
}

let wrapper: VueWrapper<any> | null = null;
async function mountResults(props: Record<string, unknown> = {}) {
  wrapper = mount(ProductSearchResults, {
    props: { search: "kids", active: true, gridMode: true, ...props },
    global: { plugins: [ui] },
    attachTo: document.body,
  });
  await flushPromises();
  await new Promise((r) => setTimeout(r, 40)); // the 30 ms search debounce
  await flushPromises();
  return wrapper;
}

const q = (s: string) => document.body.querySelector<HTMLElement>(s);
const qa = (s: string) => Array.from(document.body.querySelectorAll<HTMLElement>(s));
const cards = () => qa(".product-cell");
const card = (code: string) => {
  const c = q(`[data-group="${code}"]`);
  expect(c, `result for ${code}`).toBeTruthy();
  return c!;
};
async function tap(el: HTMLElement) {
  el.click();
  await flushPromises();
}

beforeEach(() => {
  vi.clearAllMocks();
  countGroups.mockResolvedValue(3 as any);
  listVariants.mockImplementation(async (code: string) => catalogue().filter((r) => r.code === code) as any);
});
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  document.body.innerHTML = "";
});

describe("ProductSearchResults, grid", () => {
  it("shows one result per code instead of one per colour and size", async () => {
    serve(catalogue());
    await mountResults();
    expect(cards()).toHaveLength(3);
    expect(cards().map((c) => c.querySelector(".product-cell-code")!.textContent)).toEqual(["C6KSHE29-LC", "C6KSHE35-LC", "SCARF-1"]);
  });

  it("shows a multi-variant result's colours as swatches, its sizes and price", async () => {
    serve(catalogue());
    await mountResults();
    const shoes = card("C6KSHE29-LC");
    expect(shoes.classList).toContain("product-cell--group");
    expect(Array.from(shoes.querySelectorAll<HTMLElement>(".color-swatch")).map((s) => s.title)).toEqual(["Grey", "Beige", "White", "Black"]);
    expect(shoes.querySelector(".product-group-sizes")!.textContent).toBe("20-25 · 25-30");
    expect(shoes.querySelector(".product-price")!.textContent!.trim()).toBe("$29.00");
    expect(card("C6KSHE35-LC").querySelector(".product-group-sizes")!.textContent).toBe("31-36");
  });

  it("shows a price range when the variants are priced differently", async () => {
    id = 0;
    serve([row("MIX", "Red", "S", 20), row("MIX", "Red", "XL", 25)]);
    await mountResults();
    expect(card("MIX").querySelector(".product-price")!.textContent!.trim()).toBe("$20.00 – $25.00");
  });

  it("caps the swatches and says how many more", async () => {
    id = 0;
    serve(["Black", "White", "Red", "Navy", "Pink", "Olive", "Camel", "Beige"].map((c) => row("MANY", c, "M")));
    await mountResults();
    const many = card("MANY");
    expect(many.querySelectorAll(".color-swatch")).toHaveLength(6);
    expect(many.querySelector(".product-group-more")!.textContent).toBe("+2");
  });

  it("shows a single-variant product as before, with its colour swatch", async () => {
    serve(catalogue());
    await mountResults();
    const scarf = card("SCARF-1");
    expect(scarf.classList).not.toContain("product-cell--group");
    expect(scarf.querySelector(".product-cell-variant")!.textContent!.trim()).toBe("Red");
    expect(scarf.querySelector<HTMLElement>(".color-swatch")!.title).toBe("Red");
    expect(scarf.querySelector(".product-price")!.textContent!.trim()).toBe("$12.00");
  });

  it("adds a single-variant product at once", async () => {
    serve(catalogue());
    const w = await mountResults();
    await tap(card("SCARF-1"));
    expect(q(".variant-picker")).toBeNull();
    expect(w.emitted("select")![0]![0]).toMatchObject({ code: "SCARF-1", color: "Red", qty: 1 });
  });

  it("opens the picker for a multi-variant code and adds the chosen colour and size", async () => {
    serve(catalogue());
    const w = await mountResults();
    await tap(card("C6KSHE29-LC"));
    expect(q(".variant-picker")).toBeTruthy();
    expect(q(".variant-picker-code")!.textContent).toBe("C6KSHE29-LC");
    expect(listVariants).toHaveBeenCalledWith("C6KSHE29-LC");
    expect(w.emitted("select")).toBeUndefined();

    const opt = (cls: string, text: string) => qa(cls).find((b) => b.textContent!.trim() === text)!;
    await tap(opt(".variant-option--color", "Black"));
    await tap(opt(".variant-option--size", "25-30"));
    await tap(q(".variant-picker-add")!);
    expect(w.emitted("select")).toHaveLength(1);
    expect(w.emitted("select")![0]![0]).toMatchObject({ id: 13413, code: "C6KSHE29-LC", color: "Black", size: "25-30", qty: 1 });
    expect(q(".variant-picker")).toBeNull();
  });

  it("joins a code's variants that come on a later page into the same result", async () => {
    serve(catalogue(), 5); // the 8 shoe variants are split over two pages
    await mountResults();
    // Scrolling to the end pages the rest in (the fake DOM has no layout,
    // so every scroll reads as at the bottom).
    for (let i = 0; i < 5; i++) {
      q(".product-virtual-scroll")!.dispatchEvent(new Event("scroll"));
      await flushPromises();
    }
    expect(listCursor.mock.calls.length).toBeGreaterThanOrEqual(3);
    expect(q(".product-list-status")!.textContent).toContain("all loaded");
    expect(cards().map((c) => c.dataset.group)).toEqual(["C6KSHE29-LC", "C6KSHE35-LC", "SCARF-1"]);
    expect(card("C6KSHE29-LC").querySelector(".product-group-sizes")!.textContent).toBe("20-25 · 25-30");
  });

  it("counts products, not variants", async () => {
    serve(catalogue());
    await mountResults();
    expect(countGroups).toHaveBeenCalledWith("kids");
    expect(q(".product-list-status")!.textContent!.trim()).toBe("3 products · all loaded");
  });

  it("says 1 product in the singular and none when nothing matches", async () => {
    countGroups.mockResolvedValue(1 as any);
    id = 0;
    serve([row("ONE", "Red", "S"), row("ONE", "Blue", "S")]);
    await mountResults();
    expect(q(".product-list-status")!.textContent!.trim()).toBe("1 product · all loaded");
    wrapper!.unmount();
    document.body.innerHTML = "";

    countGroups.mockResolvedValue(0 as any);
    serve([]);
    await mountResults();
    expect(q(".product-list-status")!.textContent!.trim()).toBe("No products found");
  });

  it("leaves out swatches and sizes the branch hides", async () => {
    serve(catalogue());
    await mountResults({ hiddenFields: ["color", "size"] });
    const shoes = card("C6KSHE29-LC");
    expect(shoes.querySelector(".color-swatch")).toBeNull();
    expect(shoes.querySelector(".product-group-sizes")).toBeNull();
    // The picker still asks: the cashier has to choose one.
    await tap(shoes);
    expect(qa(".variant-option--color")).toHaveLength(4);
  });
});

describe("ProductSearchResults, list", () => {
  it("also shows one row per code, with swatches and sizes", async () => {
    serve(catalogue());
    await mountResults({ gridMode: false });
    const rows = qa(".product-list-item");
    expect(rows.map((r) => r.dataset.group)).toEqual(["C6KSHE29-LC", "C6KSHE35-LC", "SCARF-1"]);
    expect(rows[0]!.classList).toContain("product-list-item--group");
    expect(rows[0]!.querySelectorAll(".color-swatch")).toHaveLength(4);
    expect(rows[0]!.querySelector(".product-group-sizes")!.textContent).toBe("20-25 · 25-30");
  });

  it("opens the picker from a row", async () => {
    serve(catalogue());
    const w = await mountResults({ gridMode: false });
    await tap(q('[data-group="C6KSHE35-LC"]')!);
    // One size: picked already, so one colour tap is all.
    await tap(qa(".variant-option--color").find((b) => b.textContent!.trim() === "White")!);
    await tap(q(".variant-picker-add")!);
    expect(w.emitted("select")![0]![0]).toMatchObject({ code: "C6KSHE35-LC", color: "White", size: "31-36" });
  });
});
