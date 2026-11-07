// @vitest-environment happy-dom
//
// The colour/size picker for a code sold in several variants, with the real
// UI kit and the backend mocked.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

vi.mock("../../wailsjs/go/main/App", () => ({ ListProductVariants: vi.fn() }));

import { ListProductVariants } from "../../wailsjs/go/main/App";
import VariantPicker from "./VariantPicker.vue";

const listVariants = vi.mocked(ListProductVariants);

function deferred<T>() {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => ((resolve = res), (reject = rej)));
  return { promise, resolve, reject };
}

type Row = { id: number; code: string; name: string; barcode: string; color: string; size: string; price: number };
let id = 0;
const row = (code: string, color: string, size: string, price = 29, extra: Partial<Row> = {}): Row => {
  id++;
  return { id, code, name: "Kids Casual shoes", barcode: `${code}-${id}`, color, size, price, ...extra };
};
function shoes(): Row[] {
  id = 13405;
  const out: Row[] = [];
  for (const size of ["20-25", "25-30"]) for (const color of ["Grey", "Beige", "White", "Black"]) out.push(row("C6KSHE29-LC", color, size));
  return out;
}

let wrapper: VueWrapper<any> | null = null;
function open(variants: Row[], opts: { refresh?: boolean; code?: string } = {}) {
  const w = mount(VariantPicker, {
    props: {
      modelValue: true,
      code: opts.code ?? variants[0]?.code ?? "",
      variants,
      refresh: opts.refresh ?? false,
      "onUpdate:modelValue": (value: boolean) => void w.setProps({ modelValue: value }),
    },
    global: { plugins: [ui] },
    attachTo: document.body,
  });
  wrapper = w;
  return w;
}

const q = (s: string) => document.body.querySelector<HTMLElement>(s);
const qa = (s: string) => Array.from(document.body.querySelectorAll<HTMLElement>(s));
const colorButtons = () => qa(".variant-option--color");
const sizeButtons = () => qa(".variant-option--size");
const labels = (els: HTMLElement[]) => els.map((e) => e.textContent!.trim());
const button = (els: HTMLElement[], text: string) => {
  const b = els.find((e) => e.textContent!.trim() === text);
  expect(b, `option "${text}" among ${JSON.stringify(labels(els))}`).toBeTruthy();
  return b!;
};
const addBtn = () => q(".variant-picker-add") as HTMLButtonElement;
// The footer's parts (choice, price) as words, joined by spaces.
const summary = () =>
  Array.from(q(".variant-picker-summary")!.children)
    .map((c) => c.textContent!.replace(/\s+/g, " ").trim())
    .filter(Boolean)
    .join(" ");
async function tap(el: HTMLElement) {
  el.click();
  await flushPromises();
}
const pressed = (el: HTMLElement) => el.getAttribute("aria-pressed") === "true";
const unavailable = (el: HTMLElement) => el.hasAttribute("data-unavailable");

beforeEach(() => {
  vi.clearAllMocks();
});
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  document.body.innerHTML = "";
});

describe("VariantPicker: the kids' shoes from the till", () => {
  it("shows the code once, the name, four colours with their swatches and two sizes", async () => {
    open(shoes());
    await flushPromises();
    expect(qa(".variant-picker-code").map((e) => e.textContent)).toEqual(["C6KSHE29-LC"]);
    expect(q(".variant-picker-name")!.textContent).toBe("Kids Casual shoes");
    expect(labels(colorButtons())).toEqual(["Grey", "Beige", "White", "Black"]);
    expect(colorButtons().map((b) => b.querySelector<HTMLElement>(".color-swatch")!.dataset.stops)).toEqual([
      "#8a8a8a",
      "#d9c7a7",
      "#ffffff",
      "#111111",
    ]);
    expect(labels(sizeButtons())).toEqual(["20-25", "25-30"]);
    // No one-line-per-variant list any more.
    expect(qa(".variant-picker-item")).toHaveLength(0);
  });

  it("keeps Add off until both a colour and a size are chosen", async () => {
    open(shoes());
    await flushPromises();
    expect(addBtn().disabled).toBe(true);
    expect(summary()).toContain("Choose a color");
    await tap(button(colorButtons(), "Grey"));
    expect(addBtn().disabled).toBe(true);
    expect(summary()).toContain("Choose a size");
    await tap(button(sizeButtons(), "25-30"));
    expect(addBtn().disabled).toBe(false);
    expect(summary()).toBe("Grey · 25-30 $29.00");
  });

  it("adds exactly the chosen variant and closes", async () => {
    const w = open(shoes());
    await flushPromises();
    await tap(button(sizeButtons(), "25-30")); // size first works too
    await tap(button(colorButtons(), "White"));
    await tap(addBtn());
    const picked = w.emitted("select")![0]![0] as Row;
    expect(picked).toMatchObject({ id: 13412, color: "White", size: "25-30", barcode: "C6KSHE29-LC-13412" });
    expect(w.emitted("select")).toHaveLength(1);
    expect(w.emitted("update:modelValue")!.at(-1)).toEqual([false]);
    expect(q(".variant-picker")).toBeNull();
  });

  it("tapping a chosen option again unchooses it", async () => {
    open(shoes());
    await flushPromises();
    const grey = button(colorButtons(), "Grey");
    await tap(grey);
    expect(pressed(grey)).toBe(true);
    await tap(grey);
    expect(pressed(grey)).toBe(false);
    expect(addBtn().disabled).toBe(true);
  });

  it("Cancel closes without adding", async () => {
    const w = open(shoes());
    await flushPromises();
    await tap(button(colorButtons(), "Grey"));
    await tap(q(".variant-picker-cancel")!);
    expect(w.emitted("select")).toBeUndefined();
    expect(w.emitted("update:modelValue")!.at(-1)).toEqual([false]);
  });
});

describe("VariantPicker: combinations that don't exist", () => {
  // Navy and Pink only in 31-36; Black only in 20-25.
  const sparse = () => {
    id = 0;
    return [row("C6KSHE35-AD", "Navy", "31-36", 35), row("C6KSHE35-AD", "Pink", "31-36", 35), row("C6KSHE35-AD", "Black", "20-25", 30)];
  };

  it("greys out the sizes a chosen colour doesn't come in, and the colours a size doesn't", async () => {
    open(sparse());
    await flushPromises();
    await tap(button(colorButtons(), "Navy"));
    expect(unavailable(button(sizeButtons(), "20-25"))).toBe(true);
    expect(unavailable(button(sizeButtons(), "31-36"))).toBe(false);
    await tap(button(colorButtons(), "Navy")); // unchoose
    await tap(button(sizeButtons(), "20-25"));
    expect(colorButtons().filter(unavailable).map((b) => b.textContent!.trim())).toEqual(["Navy", "Pink"]);
  });

  it("tapping a greyed-out option switches to it and drops the clashing choice", async () => {
    open(sparse());
    await flushPromises();
    await tap(button(colorButtons(), "Navy"));
    await tap(button(sizeButtons(), "31-36"));
    expect(addBtn().disabled).toBe(false);
    await tap(button(sizeButtons(), "20-25")); // Navy has no 20-25
    expect(pressed(button(sizeButtons(), "20-25"))).toBe(true);
    expect(colorButtons().some(pressed)).toBe(false);
    expect(addBtn().disabled).toBe(true);
    await tap(button(colorButtons(), "Black"));
    expect(summary()).toBe("Black · 20-25 $30.00");
  });

  it("shows the price range while the choice is open, the exact price once made", async () => {
    open(sparse());
    await flushPromises();
    expect(summary()).toContain("$30.00 – $35.00");
    await tap(button(colorButtons(), "Pink"));
    expect(summary()).toContain("$35.00");
    expect(summary()).not.toContain("–");
  });
});

describe("VariantPicker: choices made for the cashier", () => {
  it("picks the only size, so one tap on a colour is enough", async () => {
    id = 0;
    const w = open(["Grey", "Beige", "White", "Black"].map((c) => row("C6KSHE35-LC", c, "31-36", 35)));
    await flushPromises();
    expect(pressed(button(sizeButtons(), "31-36"))).toBe(true);
    await tap(button(colorButtons(), "Black"));
    expect(addBtn().disabled).toBe(false);
    await tap(addBtn());
    expect((w.emitted("select")![0]![0] as Row).color).toBe("Black");
  });

  it("picks the only colour, and the only size can't be unchosen", async () => {
    id = 0;
    open([row("P1", "Red", "S"), row("P1", "Red", "M")]);
    await flushPromises();
    const red = button(colorButtons(), "Red");
    expect(pressed(red)).toBe(true);
    await tap(red);
    expect(pressed(red)).toBe(true);
  });

  it("asks only for the size when the variants have no colour", async () => {
    id = 0;
    open([row("P2", "", "S"), row("P2", "", "M"), row("P2", "", "L")]);
    await flushPromises();
    expect(colorButtons()).toHaveLength(0);
    expect(q(".variant-picker-section .variant-picker-label")!.textContent).toBe("Size");
    expect(summary()).toContain("Choose a size");
    await tap(button(sizeButtons(), "M"));
    expect(addBtn().disabled).toBe(false);
  });
});

describe("VariantPicker: variants the choices can't tell apart", () => {
  it("lists two variants with the same colour and size to pick from", async () => {
    id = 0;
    const w = open([row("P3", "Black", "M", 20, { barcode: "AAA" }), row("P3", "Black", "M", 22, { barcode: "BBB" }), row("P3", "Red", "M")]);
    await flushPromises();
    await tap(button(colorButtons(), "Black"));
    expect(addBtn().disabled).toBe(true);
    const items = qa(".variant-picker-item");
    const parts = (i: HTMLElement) =>
      [".k-list-item-title", ".k-list-item-subtitle", ".k-list-item__append"].map((s) => i.querySelector(s)!.textContent!.trim());
    expect(items.map(parts)).toEqual([
      ["Kids Casual shoes", "AAA", "$20.00"],
      ["Kids Casual shoes", "BBB", "$22.00"],
    ]);
    await tap(items[1]!);
    expect((w.emitted("select")![0]![0] as Row).barcode).toBe("BBB");
  });

  it("lists variants with neither colour nor size", async () => {
    id = 0;
    const w = open([row("P4", "", "", 10, { name: "Mug A" }), row("P4", "", "", 12, { name: "Mug B" })]);
    await flushPromises();
    expect(colorButtons()).toHaveLength(0);
    expect(sizeButtons()).toHaveLength(0);
    const items = qa(".variant-picker-item");
    expect(items).toHaveLength(2);
    await tap(items[0]!);
    expect((w.emitted("select")![0]![0] as Row).name).toBe("Mug A");
  });
});

describe("VariantPicker: the whole set from the backend", () => {
  it("opens with what the search loaded, then shows every variant of the code", async () => {
    const all = shoes();
    const later = deferred<any[]>();
    listVariants.mockReturnValue(later.promise);
    open(all.slice(0, 3), { refresh: true });
    await flushPromises();
    expect(listVariants).toHaveBeenCalledWith("C6KSHE29-LC");
    expect(labels(colorButtons())).toEqual(["Grey", "Beige", "White"]);
    expect(q(".variant-picker .k-progress-circular")).toBeTruthy();

    later.resolve(all);
    await flushPromises();
    expect(labels(colorButtons())).toEqual(["Grey", "Beige", "White", "Black"]);
    expect(labels(sizeButtons())).toEqual(["20-25", "25-30"]);
    expect(q(".variant-picker .k-progress-circular")).toBeNull();
  });

  it("keeps the cashier's choice when the full set arrives", async () => {
    const all = shoes();
    const later = deferred<any[]>();
    listVariants.mockReturnValue(later.promise);
    open(all.slice(0, 4), { refresh: true }); // only 20-25: auto-picked
    await flushPromises();
    await tap(button(colorButtons(), "Beige"));
    later.resolve(all);
    await flushPromises();
    expect(pressed(button(colorButtons(), "Beige"))).toBe(true);
    // Now there are two sizes; the auto-picked one stays chosen.
    expect(pressed(button(sizeButtons(), "20-25"))).toBe(true);
    expect(addBtn().disabled).toBe(false);
  });

  it("keeps what it has if the backend fails or finds nothing", async () => {
    listVariants.mockRejectedValueOnce(new Error("db"));
    open(shoes().slice(0, 2), { refresh: true });
    await flushPromises();
    expect(labels(colorButtons())).toEqual(["Grey", "Beige"]);
    wrapper!.unmount();
    document.body.innerHTML = "";

    listVariants.mockResolvedValueOnce([]);
    open(shoes().slice(0, 2), { refresh: true });
    await flushPromises();
    expect(labels(colorButtons())).toEqual(["Grey", "Beige"]);
  });

  it("drops an answer that comes after the picker closed", async () => {
    const later = deferred<any[]>();
    listVariants.mockReturnValue(later.promise);
    const w = open(shoes().slice(0, 2), { refresh: true });
    await flushPromises();
    await tap(q(".variant-picker-cancel")!);
    later.resolve(shoes());
    await flushPromises();
    expect(q(".variant-picker")).toBeNull();
    expect(w.emitted("select")).toBeUndefined();
  });

  it("a late answer for the previous code can't replace the next code's options", async () => {
    const first = deferred<any[]>();
    listVariants.mockReturnValueOnce(first.promise);
    const w = open(shoes().slice(0, 2), { refresh: true });
    await flushPromises();
    await tap(q(".variant-picker-cancel")!);

    id = 0;
    const navy = ["Navy", "Pink"].map((c) => row("C6KSHE35-AD", c, "31-36"));
    listVariants.mockResolvedValueOnce(navy);
    await w.setProps({ code: "C6KSHE35-AD", variants: navy, modelValue: true });
    await flushPromises();
    first.resolve(shoes()); // the kids' shoes answer finally arrives
    await flushPromises();
    expect(labels(colorButtons())).toEqual(["Navy", "Pink"]);
  });

  it("offers sizes in wearing order whatever order the variants came in", async () => {
    id = 0;
    open(["XL", "S", "L", "M"].map((sz) => row("TEE", "Black", sz)));
    await flushPromises();
    expect(labels(sizeButtons())).toEqual(["S", "M", "L", "XL"]);
  });

  it("doesn't ask the backend without refresh (the scan already has every variant)", async () => {
    open(shoes());
    await flushPromises();
    expect(listVariants).not.toHaveBeenCalled();
  });

  it("starts fresh each time it opens", async () => {
    const w = open(shoes());
    await flushPromises();
    await tap(button(colorButtons(), "Grey"));
    await tap(q(".variant-picker-cancel")!);
    id = 0;
    await w.setProps({ code: "C6KSHE35-LC", variants: ["Navy", "Pink"].map((c) => row("C6KSHE35-LC", c, "31-36")), modelValue: true });
    await flushPromises();
    expect(labels(colorButtons())).toEqual(["Navy", "Pink"]);
    expect(colorButtons().some(pressed)).toBe(false);
    expect(q(".variant-picker-code")!.textContent).toBe("C6KSHE35-LC");
  });
});
