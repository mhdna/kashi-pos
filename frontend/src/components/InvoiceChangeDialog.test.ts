// @vitest-environment happy-dom
//
// UI tests for the invoice list's Change Salesperson / Change Client dialog,
// with the real UI kit and the Wails backend mocked. Backend calls are held
// open with deferred promises so each test decides when "the backend"
// answers: that is how the slow paths (first open, a search in flight) are
// pinned down.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

vi.mock("../../wailsjs/go/main/App", () => ({
  ListSalespersons: vi.fn(),
  SearchClients: vi.fn(),
  ChangeInvoiceClient: vi.fn(),
  ChangeInvoiceSalesperson: vi.fn(),
}));

import { ChangeInvoiceClient, ChangeInvoiceSalesperson, ListSalespersons, SearchClients } from "../../wailsjs/go/main/App";
import { prefetchInvoiceEditOptions, resetInvoiceEditOptions, type ChangeableInvoice } from "@/lib/invoiceEditOptions";
import InvoiceChangeDialog from "./InvoiceChangeDialog.vue";

const listSalespersons = vi.mocked(ListSalespersons);
const searchClients = vi.mocked(SearchClients);
const changeClient = vi.mocked(ChangeInvoiceClient);
const changeSalesperson = vi.mocked(ChangeInvoiceSalesperson);

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

const ROSTER = [
  { id: 3, fullName: "Rana Haddad", isSalesperson: true, isCurrent: true },
  { id: 5, fullName: "Omar Khalil", isSalesperson: true, isCurrent: true },
  { id: 8, fullName: "Door Guard", isSalesperson: false, isCurrent: true }, // attendance only
  { id: 9, fullName: "Old Till Lina", isSalesperson: true, isCurrent: false }, // till-only leftover
];

const CLIENTS = [
  { id: 11, name: "Maya Saad", phone: "+96170111222" },
  { id: 12, name: "Karim Aoun", phone: "+96178834243" },
  { id: 13, name: "No Phone", phone: "" },
];

let wrapper: VueWrapper<any> | null = null;

function mountDialog(type: "client" | "salesperson", invoice: ChangeableInvoice | null, open = true) {
  const w = mount(InvoiceChangeDialog, {
    props: {
      modelValue: open,
      type,
      invoice,
      "onUpdate:modelValue": (value: boolean) => void w.setProps({ modelValue: value }),
    },
    global: { plugins: [ui] },
    attachTo: document.body,
  });
  wrapper = w;
  return w;
}

const body = () => document.body;
const q = <T extends Element = HTMLElement>(selector: string) => body().querySelector<T>(selector);
const qa = (selector: string) => Array.from(body().querySelectorAll<HTMLElement>(selector));
const dialog = () => q(".invoice-change-dialog");
const saveBtn = () => q<HTMLButtonElement>(".invoice-change-save")!;
const cancelBtn = () => q<HTMLButtonElement>(".invoice-change-cancel")!;
const selectText = () => q(".k-select__text")?.textContent?.trim() ?? "";
const selectPlaceholder = () => q(".k-select__placeholder")?.textContent?.trim() ?? "";
// A row as the cashier reads it: the name, then " · phone" when it has one
// (the phone is the row's second line).
const rowText = (el: Element) =>
  [el.querySelector(".k-list-item-title"), el.querySelector(".k-list-item-subtitle")]
    .map((part) => part?.textContent?.trim() ?? "")
    .filter(Boolean)
    .join(" · ");
const menuTitles = () => qa(".k-menu .k-list-item").map(rowText);
const clientInput = () => q<HTMLInputElement>(".invoice-change-dialog .k-field__input")!;

async function click(el: Element | null) {
  expect(el, "element to click").toBeTruthy();
  (el as HTMLElement).click();
  await flushPromises();
}
async function openSelectMenu() {
  await click(q(".k-select__selection"));
}
async function pickMenuItem(title: string) {
  const item = qa(".k-menu .k-list-item").find((el) => rowText(el) === title);
  expect(item, `menu item "${title}" among ${JSON.stringify(menuTitles())}`).toBeTruthy();
  await click(item!);
}
async function typeInClient(text: string) {
  const input = clientInput();
  input.dispatchEvent(new Event("focus"));
  input.value = text;
  input.dispatchEvent(new Event("input"));
  await flushPromises();
}

beforeEach(() => {
  resetInvoiceEditOptions();
  vi.clearAllMocks();
  changeClient.mockResolvedValue(undefined as any);
  changeSalesperson.mockResolvedValue(undefined as any);
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  vi.useRealTimers();
  document.body.innerHTML = "";
});

// ---------------------------------------------------------------------------
describe("Change Salesperson", () => {
  const invoice: ChangeableInvoice = { id: 101, salespersonId: 3, salespersonName: "Rana Haddad" };

  it("names the current salesperson the moment it opens, before the roster arrives", async () => {
    const roster = deferred<any[]>();
    listSalespersons.mockReturnValue(roster.promise);
    mountDialog("salesperson", invoice);
    await flushPromises();

    expect(dialog()?.textContent).toContain("Change Salesperson");
    expect(selectText()).toBe("Rana Haddad");
    expect(selectText()).not.toMatch(/^\d+$/);
    expect(q(".k-select .k-progress-circular")).toBeTruthy(); // still loading…
    expect(q(".k-input--disabled")).toBeNull(); // …but not locked

    roster.resolve(ROSTER);
    await flushPromises();
    expect(selectText()).toBe("Rana Haddad");
    expect(q(".k-select .k-progress-circular")).toBeNull();
  });

  it("never shows a bare id in the field or the dropdown", async () => {
    listSalespersons.mockResolvedValue(ROSTER);
    mountDialog("salesperson", invoice);
    await flushPromises();
    await openSelectMenu();

    for (const title of [selectText(), ...menuTitles()]) expect(title).not.toMatch(/^\s*\d+\s*$/);
    expect(menuTitles()).toEqual(["Rana Haddad", "Omar Khalil"]);
  });

  it("leaves out attendance-only staff and till-only leftovers", async () => {
    listSalespersons.mockResolvedValue(ROSTER);
    mountDialog("salesperson", invoice);
    await flushPromises();
    await openSelectMenu();
    expect(menuTitles()).not.toContain("Door Guard");
    expect(menuTitles()).not.toContain("Old Till Lina");
  });

  it.each([
    ["a till-only leftover", { id: 102, salespersonId: 9, salespersonName: "Old Till Lina" }],
    ["attendance-only staff", { id: 103, salespersonId: 8, salespersonName: "Door Guard" }],
    ["someone no longer on the roster", { id: 104, salespersonId: 77, salespersonName: "Gone Person" }],
  ])("still names the invoice's salesperson when they are %s", async (_label, inv) => {
    listSalespersons.mockResolvedValue(ROSTER);
    mountDialog("salesperson", inv);
    await flushPromises();
    expect(selectText()).toBe(inv.salespersonName);
    await openSelectMenu();
    expect(menuTitles()[0]).toBe(inv.salespersonName);
    expect(menuTitles()).toContain("Rana Haddad");
  });

  it('says "Former salesperson" rather than an id when the name is unknown', async () => {
    listSalespersons.mockResolvedValue(ROSTER);
    mountDialog("salesperson", { id: 105, salespersonId: 42, salespersonName: "" });
    await flushPromises();
    expect(selectText()).toBe("Former salesperson");
    expect(body().textContent).not.toMatch(/\b42\b/);
  });

  it("shows the placeholder for an anonymous sale", async () => {
    listSalespersons.mockResolvedValue(ROSTER);
    mountDialog("salesperson", { id: 106, salespersonId: 0, salespersonName: "" });
    await flushPromises();
    expect(selectText()).toBe("");
    expect(selectPlaceholder()).toBe("Select a salesman");
  });

  it("opens with the full list straight away once loaded before (prefetch)", async () => {
    listSalespersons.mockResolvedValueOnce(ROSTER);
    prefetchInvoiceEditOptions();
    await flushPromises();

    const slow = deferred<any[]>();
    listSalespersons.mockReturnValue(slow.promise);
    mountDialog("salesperson", invoice);
    await flushPromises();
    await openSelectMenu();
    expect(menuTitles()).toEqual(["Rana Haddad", "Omar Khalil"]);

    // The fresh roster then replaces it.
    slow.resolve([...ROSTER, { id: 6, fullName: "New Hire", isSalesperson: true, isCurrent: true }]);
    await flushPromises();
    expect(menuTitles()).toEqual(["Rana Haddad", "Omar Khalil", "New Hire"]);
  });

  it("can pick and save while the fresh roster is still loading", async () => {
    listSalespersons.mockResolvedValueOnce(ROSTER);
    prefetchInvoiceEditOptions();
    await flushPromises();
    listSalespersons.mockReturnValue(new Promise(() => {}));

    const w = mountDialog("salesperson", invoice);
    await flushPromises();
    await openSelectMenu();
    await pickMenuItem("Omar Khalil");
    expect(selectText()).toBe("Omar Khalil");
    expect(saveBtn().disabled).toBe(false);
    await click(saveBtn());
    expect(changeSalesperson).toHaveBeenCalledWith(101, 5);
    expect(w.emitted("saved")).toHaveLength(1);
  });

  it("keeps Save off until the salesperson actually changes", async () => {
    listSalespersons.mockResolvedValue(ROSTER);
    mountDialog("salesperson", invoice);
    await flushPromises();
    expect(saveBtn().disabled).toBe(true);
    await openSelectMenu();
    await pickMenuItem("Omar Khalil");
    expect(saveBtn().disabled).toBe(false);
    await openSelectMenu();
    await pickMenuItem("Rana Haddad");
    expect(saveBtn().disabled).toBe(true);
  });

  it("saves the picked salesperson, then reports and closes", async () => {
    listSalespersons.mockResolvedValue(ROSTER);
    const w = mountDialog("salesperson", invoice);
    await flushPromises();
    await openSelectMenu();
    await pickMenuItem("Omar Khalil");
    await click(saveBtn());

    expect(changeSalesperson).toHaveBeenCalledTimes(1);
    expect(changeSalesperson).toHaveBeenCalledWith(101, 5);
    expect(changeClient).not.toHaveBeenCalled();
    expect(w.emitted("saved")).toHaveLength(1);
    expect(w.emitted("update:modelValue")?.at(-1)).toEqual([false]);
    expect(dialog()).toBeNull();
  });

  it("clearing the field saves the sale as anonymous (0)", async () => {
    listSalespersons.mockResolvedValue(ROSTER);
    mountDialog("salesperson", invoice);
    await flushPromises();
    await click(q(".k-field__clear .k-field__icon-btn"));
    expect(selectPlaceholder()).toBe("Select a salesman");
    await click(saveBtn());
    expect(changeSalesperson).toHaveBeenCalledWith(101, 0);
  });

  it("picks with the arrow keys", async () => {
    listSalespersons.mockResolvedValue(ROSTER);
    mountDialog("salesperson", invoice);
    await flushPromises();
    q(".k-select__selection")!.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
    await flushPromises();
    expect(selectText()).toBe("Omar Khalil");
    expect(saveBtn().disabled).toBe(false);
  });

  it("disables Save while saving so it can't be sent twice", async () => {
    listSalespersons.mockResolvedValue(ROSTER);
    const saving = deferred<void>();
    changeSalesperson.mockReturnValue(saving.promise as any);
    mountDialog("salesperson", invoice);
    await flushPromises();
    await openSelectMenu();
    await pickMenuItem("Omar Khalil");
    await click(saveBtn());
    expect(saveBtn().disabled).toBe(true);
    expect(saveBtn().className).toContain("k-btn--loading");
    await click(saveBtn());
    expect(changeSalesperson).toHaveBeenCalledTimes(1);
    saving.resolve();
    await flushPromises();
  });

  it("stays open and reports when saving fails", async () => {
    listSalespersons.mockResolvedValue(ROSTER);
    changeSalesperson.mockRejectedValue("salesperson not found");
    const w = mountDialog("salesperson", invoice);
    await flushPromises();
    await openSelectMenu();
    await pickMenuItem("Omar Khalil");
    await click(saveBtn());
    expect(w.emitted("error")).toEqual([["salesperson not found"]]);
    expect(w.emitted("saved")).toBeUndefined();
    expect(dialog()).toBeTruthy();
    expect(saveBtn().disabled).toBe(false);
  });

  it("reports a roster that fails to load but still names the current salesperson", async () => {
    listSalespersons.mockRejectedValue("database not initialized");
    const w = mountDialog("salesperson", invoice);
    await flushPromises();
    expect(w.emitted("error")).toEqual([["database not initialized"]]);
    expect(selectText()).toBe("Rana Haddad");
  });

  it("Cancel closes without saving", async () => {
    listSalespersons.mockResolvedValue(ROSTER);
    const w = mountDialog("salesperson", invoice);
    await flushPromises();
    await openSelectMenu();
    await pickMenuItem("Omar Khalil");
    await click(cancelBtn());
    expect(changeSalesperson).not.toHaveBeenCalled();
    expect(w.emitted("update:modelValue")?.at(-1)).toEqual([false]);
  });

  it("reopening for another invoice starts from that invoice's salesperson", async () => {
    listSalespersons.mockResolvedValue(ROSTER);
    const w = mountDialog("salesperson", invoice);
    await flushPromises();
    await openSelectMenu();
    await pickMenuItem("Omar Khalil");
    await click(cancelBtn());

    await w.setProps({ invoice: { id: 200, salespersonId: 9, salespersonName: "Old Till Lina" }, modelValue: true });
    await flushPromises();
    expect(selectText()).toBe("Old Till Lina");
    expect(saveBtn().disabled).toBe(true);
  });
});

// ---------------------------------------------------------------------------
describe("Change Client", () => {
  const invoice: ChangeableInvoice = { id: 301, clientId: 12, clientName: "Karim Aoun" };

  it("names the current client the moment it opens, before any search returns", async () => {
    searchClients.mockReturnValue(new Promise(() => {}));
    mountDialog("client", invoice);
    await flushPromises();
    expect(dialog()?.textContent).toContain("Change Client");
    expect(clientInput().value).toBe("Karim Aoun");
    expect(q(".invoice-change-dialog .k-progress-circular")).toBeTruthy();
  });

  it("asks the backend for the first page once on open", async () => {
    searchClients.mockResolvedValue(CLIENTS);
    mountDialog("client", invoice);
    await flushPromises();
    expect(searchClients).toHaveBeenCalledTimes(1);
    expect(searchClients).toHaveBeenCalledWith("", 0);
    expect(clientInput().value).toBe("Karim Aoun · (78) 834-243");
  });

  it("lists clients with formatted phones", async () => {
    searchClients.mockResolvedValue(CLIENTS);
    mountDialog("client", invoice);
    await flushPromises();
    clientInput().dispatchEvent(new Event("focus"));
    await flushPromises();
    expect(menuTitles()).toEqual(["Maya Saad · (70) 111-222", "Karim Aoun · (78) 834-243", "No Phone"]);
  });

  it("opens with the list straight away once loaded before (prefetch)", async () => {
    searchClients.mockResolvedValueOnce(CLIENTS);
    prefetchInvoiceEditOptions();
    await flushPromises();
    searchClients.mockReturnValue(new Promise(() => {}));

    mountDialog("client", invoice);
    await flushPromises();
    clientInput().dispatchEvent(new Event("focus"));
    await flushPromises();
    expect(menuTitles()).toHaveLength(3);
    expect(clientInput().value).toBe("Karim Aoun · (78) 834-243");
  });

  it("keeps the current client on offer when they aren't on the first page", async () => {
    searchClients.mockResolvedValue(CLIENTS.filter((c) => c.id !== 12));
    mountDialog("client", invoice);
    await flushPromises();
    expect(clientInput().value).toBe("Karim Aoun");
    clientInput().dispatchEvent(new Event("focus"));
    await flushPromises();
    expect(menuTitles()[0]).toBe("Karim Aoun");
  });

  it("debounces typing into one search per pause", async () => {
    vi.useFakeTimers();
    searchClients.mockResolvedValue(CLIENTS);
    mountDialog("client", invoice);
    await flushPromises();
    searchClients.mockClear();

    for (const text of ["m", "ma", "may"]) {
      await typeInClient(text);
      vi.advanceTimersByTime(50);
    }
    expect(searchClients).not.toHaveBeenCalled();
    vi.advanceTimersByTime(150);
    await flushPromises();
    expect(searchClients).toHaveBeenCalledTimes(1);
    expect(searchClients).toHaveBeenCalledWith("may", 0);
  });

  it("shows the search results", async () => {
    vi.useFakeTimers();
    searchClients.mockResolvedValueOnce(CLIENTS);
    mountDialog("client", invoice);
    await flushPromises();
    searchClients.mockResolvedValueOnce([{ id: 40, name: "Mayssa Farah", phone: "" }]);
    await typeInClient("mays");
    vi.advanceTimersByTime(150);
    await flushPromises();
    expect(menuTitles()).toEqual(["Karim Aoun · (78) 834-243", "Mayssa Farah"]);
  });

  it("ignores a slow older search that answers after a newer one", async () => {
    vi.useFakeTimers();
    searchClients.mockResolvedValueOnce(CLIENTS);
    mountDialog("client", { id: 302, clientId: 0, clientName: "" });
    await flushPromises();

    const older = deferred<any[]>();
    const newer = deferred<any[]>();
    searchClients.mockReturnValueOnce(older.promise).mockReturnValueOnce(newer.promise);
    await typeInClient("ma");
    vi.advanceTimersByTime(150);
    await typeInClient("maya");
    vi.advanceTimersByTime(150);
    newer.resolve([CLIENTS[0]]);
    await flushPromises();
    older.resolve(CLIENTS);
    await flushPromises();
    expect(menuTitles()).toEqual(["Maya Saad · (70) 111-222"]);
  });

  it("enables Save the moment a client is picked, without another search", async () => {
    vi.useFakeTimers();
    searchClients.mockResolvedValue(CLIENTS);
    mountDialog("client", invoice);
    await flushPromises();
    searchClients.mockClear();

    clientInput().dispatchEvent(new Event("focus"));
    await flushPromises();
    await pickMenuItem("Maya Saad · (70) 111-222");
    expect(clientInput().value).toBe("Maya Saad · (70) 111-222");
    expect(saveBtn().disabled).toBe(false);
    vi.advanceTimersByTime(1000);
    await flushPromises();
    expect(searchClients).not.toHaveBeenCalled();
  });

  it("enables Save after a pick even while a search is in flight", async () => {
    vi.useFakeTimers();
    searchClients.mockResolvedValueOnce(CLIENTS);
    mountDialog("client", invoice);
    await flushPromises();
    searchClients.mockReturnValue(new Promise(() => {}));
    await typeInClient("ma");
    vi.advanceTimersByTime(150);
    await flushPromises();
    expect(q(".invoice-change-dialog .k-progress-circular")).toBeTruthy();
    await pickMenuItem("Maya Saad · (70) 111-222");
    expect(saveBtn().disabled).toBe(false);
  });

  it("keeps Save off until a different client is picked", async () => {
    searchClients.mockResolvedValue(CLIENTS);
    mountDialog("client", invoice);
    await flushPromises();
    expect(saveBtn().disabled).toBe(true);
    clientInput().dispatchEvent(new Event("focus"));
    await flushPromises();
    await pickMenuItem("Karim Aoun · (78) 834-243");
    expect(saveBtn().disabled).toBe(true);
  });

  it("keeps Save off when the invoice has no client and none is picked", async () => {
    searchClients.mockResolvedValue(CLIENTS);
    mountDialog("client", { id: 303, clientId: 0, clientName: "" });
    await flushPromises();
    expect(clientInput().value).toBe("");
    expect(saveBtn().disabled).toBe(true);
  });

  it("saves the picked client, then reports and closes", async () => {
    searchClients.mockResolvedValue(CLIENTS);
    const w = mountDialog("client", invoice);
    await flushPromises();
    clientInput().dispatchEvent(new Event("focus"));
    await flushPromises();
    await pickMenuItem("No Phone");
    await click(saveBtn());
    expect(changeClient).toHaveBeenCalledTimes(1);
    expect(changeClient).toHaveBeenCalledWith(301, 13);
    expect(changeSalesperson).not.toHaveBeenCalled();
    expect(w.emitted("saved")).toHaveLength(1);
    expect(dialog()).toBeNull();
  });

  it("stays open and reports when saving fails", async () => {
    searchClients.mockResolvedValue(CLIENTS);
    changeClient.mockRejectedValue("a named client is required for an on-account invoice");
    const w = mountDialog("client", invoice);
    await flushPromises();
    clientInput().dispatchEvent(new Event("focus"));
    await flushPromises();
    await pickMenuItem("Maya Saad · (70) 111-222");
    await click(saveBtn());
    expect(w.emitted("error")).toEqual([["a named client is required for an on-account invoice"]]);
    expect(dialog()).toBeTruthy();
  });

  it("reports a failed search", async () => {
    searchClients.mockRejectedValue(new Error("boom"));
    const w = mountDialog("client", invoice);
    await flushPromises();
    expect(w.emitted("error")).toEqual([["Could not search clients"]]);
    expect(clientInput().value).toBe("Karim Aoun");
  });

  it("drops a search that answers after the dialog closed", async () => {
    const late = deferred<any[]>();
    searchClients.mockReturnValue(late.promise);
    const w = mountDialog("client", invoice);
    await flushPromises();
    await click(cancelBtn());
    late.resolve(CLIENTS);
    await flushPromises();
    expect(w.emitted("error")).toBeUndefined();
    expect(dialog()).toBeNull();
  });

  it("reopening for another invoice starts from that invoice's client", async () => {
    searchClients.mockResolvedValue(CLIENTS);
    const w = mountDialog("client", invoice);
    await flushPromises();
    clientInput().dispatchEvent(new Event("focus"));
    await flushPromises();
    await pickMenuItem("Maya Saad · (70) 111-222");
    await click(cancelBtn());

    await w.setProps({ invoice: { id: 400, clientId: 13, clientName: "No Phone" }, modelValue: true });
    await flushPromises();
    expect(clientInput().value).toBe("No Phone");
    expect(saveBtn().disabled).toBe(true);
  });
});

// ---------------------------------------------------------------------------
describe("the dialog's look: who it is now, and avatars", () => {
  // The "Now" card read in order: avatar initials, name, then phone or note.
  const now = () =>
    qa('[data-test="change-current"] .k-avatar, [data-test="change-current"] .invoice-change-current__text > div')
      .map((el) => el.textContent?.trim() ?? "")
      .filter(Boolean)
      .join(" ");
  const rowInitials = () => qa(".k-menu .k-list-item .k-avatar").map((el) => el.textContent?.trim());

  it("names the current salesperson with their initials, before the roster arrives", async () => {
    listSalespersons.mockReturnValue(deferred<any[]>().promise);
    mountDialog("salesperson", { id: 1, invoiceCode: "CT1-00042", salespersonId: 3, salespersonName: "Rana Haddad" });
    await flushPromises();
    expect(now()).toBe("RH Rana Haddad");
    expect(dialog()?.textContent).toContain("Invoice CT1-00042");
    expect(q(".k-select .k-avatar")?.textContent?.trim()).toBe("RH");
  });

  it("never shows a bare id as who it is now, even for a salesperson no longer offered", async () => {
    listSalespersons.mockResolvedValue(ROSTER);
    mountDialog("salesperson", { id: 1, salespersonId: 9, salespersonName: "" });
    await flushPromises();
    expect(now()).toBe("FS Former salesperson");
    expect(now()).not.toMatch(/\b9\b/);
  });

  it("says Anonymous sale when the invoice has no salesperson", async () => {
    listSalespersons.mockResolvedValue(ROSTER);
    mountDialog("salesperson", { id: 1, salespersonId: 0 });
    await flushPromises();
    expect(now()).toBe("Anonymous sale");
  });

  it("gives every salesperson row an avatar with their initials", async () => {
    listSalespersons.mockResolvedValue(ROSTER);
    mountDialog("salesperson", { id: 1, salespersonId: 3, salespersonName: "Rana Haddad" });
    await flushPromises();
    await openSelectMenu();
    expect(rowInitials()).toEqual(["RH", "OK"]);
  });

  it("moves the field's avatar to the salesperson picked", async () => {
    listSalespersons.mockResolvedValue(ROSTER);
    mountDialog("salesperson", { id: 1, salespersonId: 3, salespersonName: "Rana Haddad" });
    await flushPromises();
    await openSelectMenu();
    await pickMenuItem("Omar Khalil");
    expect(q(".k-select .k-avatar")?.textContent?.trim()).toBe("OK");
    expect(now()).toBe("RH Rana Haddad"); // who it is now doesn't move until saved
  });

  it("names the current client with their phone", async () => {
    searchClients.mockResolvedValue(CLIENTS);
    mountDialog("client", { id: 1, clientId: 12, clientName: "Karim Aoun", clientPhone: "+96178834243" });
    await flushPromises();
    expect(now()).toBe("KA Karim Aoun (78) 834-243");
  });

  it("says Anonymous when the invoice has no client", async () => {
    searchClients.mockResolvedValue(CLIENTS);
    mountDialog("client", { id: 1, clientId: 0 });
    await flushPromises();
    expect(now()).toBe("Anonymous");
  });

  it("says Anonymous, with no avatar or phone, for the walk-in client", async () => {
    searchClients.mockResolvedValue(CLIENTS);
    mountDialog("client", { id: 1, clientId: 5, clientName: "Walk-in", clientPhone: "00000000", clientIsWalkIn: true });
    await flushPromises();
    expect(now()).toBe("Anonymous");
  });

  it("gives every client row an avatar, the phone on its second line", async () => {
    searchClients.mockResolvedValue(CLIENTS);
    mountDialog("client", { id: 1, clientId: 0 });
    await flushPromises();
    clientInput().dispatchEvent(new Event("focus"));
    await flushPromises();
    expect(rowInitials()).toEqual(["MS", "KA", "NP"]);
    expect(qa(".k-menu .k-list-item-subtitle").map((el) => el.textContent?.trim())).toEqual(["(70) 111-222", "(78) 834-243"]);
  });
});
