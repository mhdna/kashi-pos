// @vitest-environment happy-dom
//
// The invoice list's "Change Salesperson" dialog (whole invoice + each item), with the real UI kit and
// the Wails backend mocked: what the cashier sees, taps and saves.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

vi.mock("../../wailsjs/go/main/App", () => ({
  ListSalespersons: vi.fn(),
  GetInvoiceDetails: vi.fn(),
  ChangeInvoiceLineSalesperson: vi.fn(),
  ChangeInvoiceSalesperson: vi.fn(),
}));

import { ChangeInvoiceLineSalesperson, ChangeInvoiceSalesperson, GetInvoiceDetails, ListSalespersons } from "../../wailsjs/go/main/App";
import { resetInvoiceEditOptions } from "@/lib/invoiceEditOptions";
import InvoiceLineSalespersonDialog from "./InvoiceLineSalespersonDialog.vue";

const details = vi.mocked(GetInvoiceDetails);
const changeLine = vi.mocked(ChangeInvoiceLineSalesperson);
const changeInvoice = vi.mocked(ChangeInvoiceSalesperson);

const ROSTER = [
  { id: 3, fullName: "Rana Haddad", isSalesperson: true, isCurrent: true },
  { id: 5, fullName: "Omar Khalil", isSalesperson: true, isCurrent: true },
  { id: 8, fullName: "Door Guard", isSalesperson: false, isCurrent: true },
];
const ITEMS = [
  { lineId: 10, name: "Shirt", code: "SH1", qty: 1, salespersonId: 0, salespersonName: "" },
  { lineId: 11, name: "Jeans", code: "JN2", qty: 2, salespersonId: 3, salespersonName: "Rana Haddad" },
  { lineId: 12, name: "Scarf", code: "SC3", qty: 1, salespersonId: 77, salespersonName: "Gone Person" },
];
const INVOICE = { id: 101, invoiceCode: "SA/TB/2026/00033", salespersonId: 5, salespersonName: "Omar Khalil" };

let wrapper: VueWrapper<any> | null = null;
beforeEach(() => {
  resetInvoiceEditOptions();
  vi.mocked(ListSalespersons).mockResolvedValue(ROSTER as any);
  details.mockReset();
  details.mockResolvedValue({ items: ITEMS } as any);
  changeLine.mockReset();
  changeLine.mockResolvedValue(undefined as any);
  changeInvoice.mockReset();
  changeInvoice.mockResolvedValue(undefined as any);
});
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  document.body.innerHTML = "";
});

async function mountDialog() {
  const w = mount(InvoiceLineSalespersonDialog, {
    props: { modelValue: true, invoice: INVOICE, "onUpdate:modelValue": (v: boolean) => void w.setProps({ modelValue: v }) },
    global: { plugins: [ui] },
    attachTo: document.body,
  });
  wrapper = w;
  await flushPromises();
  return w;
}
const q = (s: string) => document.body.querySelector<HTMLElement>(s);
const qa = (s: string) => Array.from(document.body.querySelectorAll<HTMLElement>(s));
const click = async (el: Element | null | undefined) => {
  el!.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  await flushPromises();
};
const save = () => q(".line-sp-save") as HTMLButtonElement;
const who = (lineId: number) => q(`[data-test=line-${lineId}] .line-sp-who__name`)!.textContent!.trim();

describe("Change Salesperson (invoice and items)", () => {
  it("lists every item with who sold it, or the invoice's salesperson when nobody", async () => {
    await mountDialog();
    expect(qa("[data-test^=line-] .line-sp-row__name").map((n) => n.textContent)).toEqual(["Shirt", "Jeans", "Scarf"]);
    expect(who(10)).toBe("Invoice: Omar Khalil");
    expect(who(11)).toBe("Rana Haddad");
    expect(who(12)).toBe("Gone Person"); // no longer on the roster, still named
    expect(details).toHaveBeenCalledWith(101);
  });

  it("keeps Save off until something changes", async () => {
    await mountDialog();
    expect(save().disabled).toBe(true);
    await click(q("[data-test=line-10]"));
    expect(save().disabled).toBe(true); // opening the grid changes nothing
  });

  it("opens one grid at a time, with real salespeople and a same-as-invoice tile", async () => {
    await mountDialog();
    await click(q("[data-test=line-10]"));
    expect(qa(".line-sp-picker")).toHaveLength(1);
    expect(q("[data-test=sp-3]")?.textContent).toContain("Rana Haddad");
    expect(q("[data-test=sp-8]")).toBeNull(); // attendance-only staff
    expect(q("[data-test=sp-default]")?.textContent).toContain("Same as invoice (Omar)");
    await click(q("[data-test=line-11]"));
    expect(qa(".line-sp-picker")).toHaveLength(1);
    await click(q("[data-test=line-11]"));
    expect(qa(".line-sp-picker")).toHaveLength(0);
  });

  it("saves only the lines that changed, then reports and closes", async () => {
    const w = await mountDialog();
    await click(q("[data-test=line-10]"));
    await click(q("[data-test=sp-5]")); // Shirt -> Omar
    expect(who(10)).toBe("Omar Khalil");
    await click(q("[data-test=line-11]"));
    await click(q("[data-test=sp-default]")); // Jeans -> back to the invoice
    await click(q("[data-test=line-12]"));
    await click(q("[data-test=sp-3]")); // Scarf -> Rana
    expect(save().disabled).toBe(false);
    await click(save());
    expect(changeLine.mock.calls).toEqual([
      [101, 10, 5],
      [101, 11, 0],
      [101, 12, 3],
    ]);
    expect(w.emitted("saved")).toHaveLength(1);
    expect(w.props("modelValue")).toBe(false);
  });

  it("does not save a line put back the way it was", async () => {
    await mountDialog();
    await click(q("[data-test=line-11]"));
    await click(q("[data-test=sp-5]"));
    await click(q("[data-test=line-11]"));
    await click(q("[data-test=sp-3]")); // back to Rana, as saved
    expect(save().disabled).toBe(true);
  });

  it("cancel closes without saving", async () => {
    const w = await mountDialog();
    await click(q("[data-test=line-10]"));
    await click(q("[data-test=sp-5]"));
    await click(q(".line-sp-cancel"));
    expect(changeLine).not.toHaveBeenCalled();
    expect(w.props("modelValue")).toBe(false);
  });

  it("reports a failed save, stays open and reloads the truth", async () => {
    const w = await mountDialog();
    await click(q("[data-test=line-10]"));
    await click(q("[data-test=sp-5]"));
    changeLine.mockRejectedValueOnce(new Error("invoice line not found"));
    await click(save());
    expect(w.emitted("error")?.[0]?.[0]).toContain("invoice line not found");
    expect(w.emitted("saved")).toBeUndefined();
    expect(w.props("modelValue")).toBe(true);
    expect(details).toHaveBeenCalledTimes(2);
  });

  describe("one item or none", () => {
    const itemsShown = () => qa("[data-test^=line-]").length;

    it("lists the items only when there are two or more", async () => {
      details.mockResolvedValue({ items: ITEMS.slice(0, 2) } as any);
      await mountDialog();
      expect(itemsShown()).toBe(2);
      expect(qa(".line-sp-label").map((l) => l.textContent)).toEqual(["Whole invoice", "Items"]);
      wrapper!.unmount();
      document.body.innerHTML = "";
      details.mockResolvedValue({ items: ITEMS.slice(0, 1) } as any);
      await mountDialog();
      expect(itemsShown()).toBe(0);
      expect(qa(".line-sp-label").map((l) => l.textContent)).toEqual(["Whole invoice"]);
    });

    it("shows just the invoice selector when there are no items", async () => {
      details.mockResolvedValue({ items: [] } as any);
      await mountDialog();
      expect(itemsShown()).toBe(0);
      expect(q("[data-test=invoice-row]")).toBeTruthy();
    });

    it("a lone item that follows the invoice: only the invoice is changed", async () => {
      details.mockResolvedValue({ items: [ITEMS[0]] } as any);
      await mountDialog();
      await click(q("[data-test=invoice-row]"));
      await click(q("[data-test=sp-3]"));
      await click(save());
      expect(changeInvoice.mock.calls).toEqual([[101, 3]]);
      expect(changeLine).not.toHaveBeenCalled();
    });

    it("a lone item set apart earlier follows the new invoice salesperson", async () => {
      details.mockResolvedValue({ items: [ITEMS[1]] } as any); // Jeans, Rana
      await mountDialog();
      await click(q("[data-test=invoice-row]"));
      await click(q("[data-test=sp-5]")); // Omar is already the invoice's: no change
      expect(save().disabled).toBe(true);
      await click(q("[data-test=invoice-row]"));
      await click(q("[data-test=sp-default]")); // No salesperson: a real change
      await click(save());
      expect(changeInvoice.mock.calls).toEqual([[101, 0]]);
      expect(changeLine.mock.calls).toEqual([[101, 11, 0]]);
    });
  });

  describe("the whole invoice", () => {
    const invoiceWho = () => q("[data-test=invoice-row] .line-sp-who__name")!.textContent!.trim();

    it("shows who the invoice is under, above the items", async () => {
      await mountDialog();
      expect(invoiceWho()).toBe("Omar Khalil");
      const labels = qa(".line-sp-label").map((l) => l.textContent);
      expect(labels).toEqual(["Whole invoice", "Items"]);
    });

    it("changes the invoice alone, leaving the items as they are", async () => {
      const w = await mountDialog();
      await click(q("[data-test=invoice-row]"));
      expect(q("[data-test=sp-8]")).toBeNull(); // attendance-only staff
      await click(q("[data-test=sp-3]"));
      expect(invoiceWho()).toBe("Rana Haddad");
      expect(save().disabled).toBe(false);
      await click(save());
      expect(changeInvoice.mock.calls).toEqual([[101, 3]]);
      expect(changeLine).not.toHaveBeenCalled();
      expect(w.emitted("saved")).toHaveLength(1);
    });

    it("clears the invoice's salesperson with the No salesperson tile", async () => {
      await mountDialog();
      await click(q("[data-test=invoice-row]"));
      await click(q("[data-test=sp-default]"));
      expect(invoiceWho()).toBe("No salesperson");
      await click(save());
      expect(changeInvoice.mock.calls).toEqual([[101, 0]]);
    });

    it("items that follow the invoice name the new person straight away", async () => {
      await mountDialog();
      await click(q("[data-test=invoice-row]"));
      await click(q("[data-test=sp-3]"));
      expect(who(10)).toBe("Invoice: Rana Haddad");
      await click(q("[data-test=line-10]"));
      expect(q("[data-test=sp-default]")?.textContent).toContain("Same as invoice (Rana)");
    });

    it("saves the invoice first, then its items; picking the same person is no change", async () => {
      await mountDialog();
      await click(q("[data-test=invoice-row]"));
      await click(q("[data-test=sp-5]")); // Omar again: nothing changed
      expect(save().disabled).toBe(true);
      await click(q("[data-test=invoice-row]"));
      await click(q("[data-test=sp-3]"));
      await click(q("[data-test=line-10]"));
      await click(q("[data-test=sp-5]"));
      const order: string[] = [];
      changeInvoice.mockImplementation(async () => void order.push("invoice"));
      changeLine.mockImplementation(async () => void order.push("line"));
      await click(save());
      expect(order).toEqual(["invoice", "line"]);
    });

    it("a refused invoice change stops before touching any item", async () => {
      const w = await mountDialog();
      await click(q("[data-test=invoice-row]"));
      await click(q("[data-test=sp-3]"));
      await click(q("[data-test=line-10]"));
      await click(q("[data-test=sp-5]"));
      changeInvoice.mockRejectedValueOnce(new Error("salesperson not found"));
      await click(save());
      expect(changeLine).not.toHaveBeenCalled();
      expect(w.emitted("error")?.[0]?.[0]).toContain("salesperson not found");
      expect(w.props("modelValue")).toBe(true);
    });
  });
});
