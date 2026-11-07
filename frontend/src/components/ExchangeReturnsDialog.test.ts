// @vitest-environment happy-dom
//
// UI tests for the exchange screen's "Returned items" picker, with the real
// UI kit and the Wails backend mocked: what the cashier sees, what tapping
// + and − does to the cart, and what it won't let them do.
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

vi.mock("../../wailsjs/go/main/App", () => ({ GetExchangeInvoiceByID: vi.fn() }));

import { GetExchangeInvoiceByID } from "../../wailsjs/go/main/App";
import { returnedCartLine } from "@/lib/exchangeReturns";
import ExchangeReturnsDialog from "./ExchangeReturnsDialog.vue";

const getSale = vi.mocked(GetExchangeInvoiceByID);

// As getExchangeInvoice sends it: a deal's free and paid units of one
// product come as two lines, merged by the picker.
const SALE = {
  id: 412,
  invoiceCode: "SA/CT/2026/02900",
  items: [
    {
      code: "BOC5KBJOP9",
      barcode: "bc-pants",
      name: "Kids Boy Jogging Pants",
      price: 9,
      discount: 0,
      available: 1,
      deal: "Buy 1 Get 1",
      free: true,
    },
    {
      code: "BOC5KBH15",
      barcode: "bc-hoodie",
      name: "Kids Boy Hoodie",
      price: 15,
      discount: 0,
      available: 2,
      color: "Navy",
      size: "8",
    },
    {
      code: "CA24KGJK29",
      barcode: "bc-jacket",
      name: "Kids Girl Jacket",
      price: 29,
      discount: 0,
      available: 1,
    },
  ],
};
const pantsLine = () => returnedCartLine({ ...SALE.items[0]!, freeQty: 1 }, 1);
const shirt = { code: "SHIRT", barcode: "bc-shirt", name: "Shirt", price: 20, qty: 1 };

let wrapper: VueWrapper<any> | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  vi.clearAllMocks();
  document.body.innerHTML = "";
});

async function open(items: any[]) {
  getSale.mockResolvedValue(SALE as any);
  wrapper = mount(ExchangeReturnsDialog, {
    props: { modelValue: true, originalInvoiceId: 412, items, roundingMode: "half_up" },
    global: { plugins: [ui] },
    attachTo: document.body,
  });
  await flushPromises();
  return wrapper;
}
const row = (code: string) =>
  [...document.querySelectorAll<HTMLElement>(".exchange-returns-row")].find((r) =>
    r.textContent?.includes(code),
  )!;
const qty = (code: string) =>
  row(code).querySelector('[data-test="returned-qty"]')!.textContent!.trim();
const state = (code: string) =>
  row(code).querySelector('[data-test="returned-state"]')!.textContent!.trim();
const plus = (code: string) =>
  row(code).querySelector<HTMLButtonElement>('[aria-label="Return one more"]')!;
const minus = (code: string) =>
  row(code).querySelector<HTMLButtonElement>('[aria-label="Return one fewer"]')!;
const lastChange = () => (wrapper!.emitted("change")?.at(-1)?.[0] ?? null) as any[] | null;

describe("ExchangeReturnsDialog", () => {
  it("loads the exchange's own sale and lists what is left on it", async () => {
    await open([pantsLine(), shirt]);
    expect(getSale).toHaveBeenCalledWith(412);
    expect(document.body.textContent).toContain("From SA/CT/2026/02900");
    expect(document.querySelectorAll(".exchange-returns-row")).toHaveLength(3);
    expect(row("BOC5KBH15").textContent).toContain(
      "BOC5KBH15 · Navy / 8 · $15.00 each · 2 on the sale",
    );
    expect(row("BOC5KBJOP9").textContent).toContain("1 free, Buy 1 Get 1");
  });

  it("shows what the cart returns now: returned and not returned", async () => {
    await open([pantsLine(), shirt]);
    expect([qty("BOC5KBJOP9"), state("BOC5KBJOP9")]).toEqual(["1", "Returned"]);
    expect([qty("BOC5KBH15"), state("BOC5KBH15")]).toEqual(["0", "Not returned"]);
  });

  it("returns an item that wasn't, with +", async () => {
    await open([pantsLine(), shirt]);
    plus("BOC5KBH15").click();
    await flushPromises();
    const cart = lastChange()!;
    expect(cart.map((l) => [l.code, l.qty])).toEqual([
      ["BOC5KBJOP9", -1],
      ["BOC5KBH15", -1],
      ["SHIRT", 1],
    ]);
    expect(cart[1]).toMatchObject({ locked: true, soldUnitPrice: 15, price: 15 });
  });

  it("puts an item back on the sale with −, while another is returned", async () => {
    const jacket = returnedCartLine(SALE.items[2]!, 1);
    await open([pantsLine(), jacket, shirt]);
    minus("BOC5KBJOP9").click();
    await flushPromises();
    expect(lastChange()!.map((l) => l.code)).toEqual(["CA24KGJK29", "SHIRT"]);
  });

  it("won't return more than the sale has left", async () => {
    await open([pantsLine(), shirt]);
    expect(plus("BOC5KBJOP9").disabled).toBe(true); // 1 of 1
    expect(plus("BOC5KBH15").disabled).toBe(false);
  });

  it("won't take back the exchange's last returned item", async () => {
    await open([pantsLine(), shirt]);
    expect(minus("BOC5KBJOP9").disabled).toBe(true);
    expect(minus("BOC5KBH15").disabled).toBe(true); // already 0
  });

  it("follows the cart as it changes", async () => {
    await open([pantsLine(), shirt]);
    await wrapper!.setProps({ items: [pantsLine(), returnedCartLine(SALE.items[1]!, 2), shirt] });
    expect(qty("BOC5KBH15")).toBe("2");
    expect(plus("BOC5KBH15").disabled).toBe(true); // 2 of 2
    expect(minus("BOC5KBJOP9").disabled).toBe(false); // the hoodies are still returned
  });

  it("says why when the sale can't be loaded, and lists nothing", async () => {
    getSale.mockRejectedValue("the exchange window for this sale has passed");
    wrapper = mount(ExchangeReturnsDialog, {
      props: { modelValue: true, originalInvoiceId: 412, items: [pantsLine()] },
      global: { plugins: [ui] },
      attachTo: document.body,
    });
    await flushPromises();
    expect(document.querySelector('[data-test="returns-error"]')!.textContent).toContain(
      "exchange window",
    );
    expect(document.querySelectorAll(".exchange-returns-row")).toHaveLength(0);
  });

  it("closes on Done without changing anything", async () => {
    await open([pantsLine(), shirt]);
    [...document.querySelectorAll("button")].find((b) => b.textContent?.trim() === "Done")!.click();
    await flushPromises();
    expect(wrapper!.emitted("update:modelValue")?.at(-1)).toEqual([false]);
    expect(wrapper!.emitted("change")).toBeUndefined();
  });
});
