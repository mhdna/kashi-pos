// @vitest-environment happy-dom
//
// UI tests for holding an exchange: the action bar lets an exchange be held
// (a settlement still can't), and the hold list shows a held exchange as one
// and hands it back whole. The real UI kit, the Wails backend mocked.
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

vi.mock("../../wailsjs/go/main/App", () => ({
  ListHoldInvoices: vi.fn(),
  DeleteHoldInvoice: vi.fn(),
}));

import { DeleteHoldInvoice, ListHoldInvoices } from "../../wailsjs/go/main/App";
import { buildHoldSnapshot } from "@/lib/holdSnapshot";
import ActionBar from "./ActionBar.vue";
import OnHoldMenu from "./OnHoldMenu.vue";

const listHolds = vi.mocked(ListHoldInvoices);
const deleteHold = vi.mocked(DeleteHoldInvoice);

const returned = { code: "CA24LWVZ12", barcode: "bc-vizon", name: "Lady Winter Vizon", price: 12, qty: -1, locked: true, soldUnitPrice: 12 };
const unknown = { code: "UNKNOWN", barcode: "UNKNOWN", name: "Unknown item", price: 30, qty: 1, unitPrices: [30] };
const socks = { code: "L6KS01.5", barcode: "bc-socks", name: "Kids Socks", price: 1.5, qty: 2 };

const heldExchange = {
  ...JSON.parse(
    JSON.stringify(
      buildHoldSnapshot({
        items: [unknown, returned],
        client: { id: 3, name: "Maya" },
        exchange: { originalInvoiceId: 412, originalInvoiceCode: "SA/CT/2026/02900" },
      }),
    ),
  ),
  id: 21,
  createdAt: "2026-11-07 10:00:00",
  itemCount: 2,
};
const heldSale = {
  ...JSON.parse(JSON.stringify(buildHoldSnapshot({ items: [socks] }))),
  id: 22,
  createdAt: "2026-11-07 10:05:00",
  itemCount: 1,
};

let wrapper: VueWrapper<any> | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  vi.clearAllMocks();
  document.body.innerHTML = "";
});

describe("ActionBar Hold", () => {
  function holdButton(mode: "sale" | "exchange" | "settlement", hasItems = true) {
    wrapper = mount(ActionBar, {
      props: { total: 24.33, totalSummary: { first: "$24.33", second: "" }, canCheckout: true, hasItems, mode },
      global: { plugins: [ui] },
      attachTo: document.body,
    });
    return wrapper.findAll("button").find((b) => b.text().includes("Hold"))!;
  }

  it("can hold a sale", () => {
    expect(holdButton("sale").attributes("disabled")).toBeUndefined();
  });
  it("can hold an exchange", () => {
    expect(holdButton("exchange").attributes("disabled")).toBeUndefined();
  });
  it("can't hold a settlement", () => {
    expect(holdButton("settlement").attributes("disabled")).toBeDefined();
  });
  it("can't hold an empty cart, sale or exchange", () => {
    expect(holdButton("sale", false).attributes("disabled")).toBeDefined();
    wrapper?.unmount();
    expect(holdButton("exchange", false).attributes("disabled")).toBeDefined();
  });
  it("asks before holding an exchange and holds it on yes", async () => {
    const button = holdButton("exchange");
    await button.trigger("click");
    await flushPromises();
    expect(document.body.textContent).toContain("Save the current invoice to hold?");
    const yes = [...document.body.querySelectorAll("button")].find((b) => /^\s*Confirm\s*$/.test(b.textContent ?? ""));
    expect(yes, "a confirm button").toBeTruthy();
    yes!.click();
    await flushPromises();
    expect(wrapper!.emitted("hold")).toHaveLength(1);
  });
});

describe("OnHoldMenu", () => {
  async function openMenu() {
    wrapper = mount(OnHoldMenu, { props: { hideActivator: true }, global: { plugins: [ui] }, attachTo: document.body });
    wrapper.vm.open();
    await flushPromises();
  }

  it("lists a held exchange as an exchange of its sale, and a sale as items", async () => {
    listHolds.mockResolvedValue([heldExchange, heldSale]);
    await openMenu();
    const text = document.body.textContent ?? "";
    expect(text).toContain("#21 — Exchange of SA/CT/2026/02900 · 1 returned, 1 new");
    expect(text).toContain("#22 — 1 item");
  });

  it("hands a held exchange back whole, returned line and sale included", async () => {
    listHolds.mockResolvedValue([heldExchange, heldSale]);
    await openMenu();
    const row = [...document.body.querySelectorAll(".on-hold-item")].find((el) => el.textContent?.includes("#21"));
    expect(row).toBeTruthy();
    (row as HTMLElement).click();
    await flushPromises();
    const restored = wrapper!.emitted("restore")?.[0]?.[0] as any;
    expect(restored.id).toBe(21);
    expect(restored.exchange).toEqual({ originalInvoiceId: 412, originalInvoiceCode: "SA/CT/2026/02900" });
    expect(restored.items.find((i: any) => i.code === "CA24LWVZ12")).toMatchObject({ qty: -1, locked: true, soldUnitPrice: 12 });
  });

  it("removes a held exchange without restoring it", async () => {
    listHolds.mockResolvedValue([heldExchange]);
    deleteHold.mockResolvedValue(undefined as any);
    await openMenu();
    const remove = document.body.querySelector('[aria-label="Remove hold"]') as HTMLElement;
    remove.click();
    await flushPromises();
    expect(deleteHold).toHaveBeenCalledWith(21);
    expect(wrapper!.emitted("restore")).toBeUndefined();
  });
});
