// @vitest-environment happy-dom
// The client dialog is one form: phone on the left, name and where they heard
// about us on the right. The options are the
// till's real GetClientSources answer (Go test TestClientSourcesFixtureForUI).
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";
import fixture from "@/lib/testdata/client_sources.json";

vi.mock("../../wailsjs/go/main/App", () => ({
  GetClientByPhone: vi.fn(),
  CreateClient: vi.fn(),
  GetClientSources: vi.fn(),
}));
vi.mock("@/lib/sound", () => ({ playClickSound: vi.fn(), playDtmfSound: vi.fn() }));

import { CreateClient, GetClientByPhone, GetClientSources } from "../../wailsjs/go/main/App";
import ClientDialog from "./ClientDialog.vue";
import { sourceLook, sourceOptions } from "@/lib/clientSource";
import { iconPathFor } from "@/lib/icons";


let wrapper: VueWrapper | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  document.body.innerHTML = "";
  vi.clearAllMocks();
});

const qa = (sel: string) => Array.from(document.body.querySelectorAll<HTMLElement>(sel));
const q = (sel: string) => document.body.querySelector<HTMLElement>(sel);
const options = () => qa('[data-test="client-source-option"]');
const option = (i: number) => options()[i]!;
const save = () => q('[data-test="client-save"]') as HTMLButtonElement;

async function openForm(sources: unknown) {
  vi.mocked(GetClientSources).mockResolvedValue(sources as never);
  vi.mocked(CreateClient).mockImplementation(async (name: string, phone: string) => ({ id: 9, name, phone }) as never);
  wrapper = mount(ClientDialog, { props: { modelValue: true }, global: { plugins: [ui] }, attachTo: document.body });
  await flushPromises();
}

async function typePhone(digits: string) {
  const phone = q(".client-phone-numpad-display-input") as HTMLInputElement;
  phone.value = digits;
  phone.dispatchEvent(new Event("input"));
  await flushPromises();
}

async function newCustomerStep(sources: unknown) {
  vi.mocked(GetClientByPhone).mockRejectedValue(new Error("not found"));
  await openForm(sources);
  await typePhone("70456789");
}

async function typeName(name: string) {
  const inputs = qa("input").filter((i) => (i as HTMLInputElement).placeholder === "Name") as HTMLInputElement[];
  inputs[0]!.value = name;
  inputs[0]!.dispatchEvent(new Event("input"));
  await flushPromises();
}

describe("ClientDialog: where did they hear about us", () => {
  it("the reported case: a new customer needs an answer, and it is saved with them", async () => {
    await newCustomerStep(fixture);
    expect(document.body.textContent).toContain("Where did they hear about us?");
    expect(options().map((o) => o.textContent?.trim())).toEqual(["Social media", "A friend or family", "Walking by"]);

    await typeName("maya haddad");
    expect(save().disabled).toBe(true);

    option(2).click();
    await flushPromises();
    expect(option(2).getAttribute("aria-checked")).toBe("true");
    expect(options().filter((o) => o.getAttribute("aria-checked") === "true")).toHaveLength(1);
    expect(save().disabled).toBe(false);

    save().click();
    await flushPromises();
    expect(CreateClient).toHaveBeenCalledWith("Maya Haddad", "+96170456789", fixture[2]!.uid);
    expect(wrapper!.emitted("select")?.[0]?.[0]).toMatchObject({ id: 9, name: "Maya Haddad" });
  });

  it("tapping the picked answer again clears it, and save waits again", async () => {
    await newCustomerStep(fixture);
    await typeName("Ali");
    option(0).click();
    await flushPromises();
    option(0).click();
    await flushPromises();
    expect(save().disabled).toBe(true);
    option(1).click();
    await flushPromises();
    save().click();
    await flushPromises();
    expect(CreateClient).toHaveBeenCalledWith("Ali", "+96170456789", fixture[1]!.uid);
  });

  it("each option is filled in its colour with its icon, and the picked one is ringed", async () => {
    await newCustomerStep(fixture);
    const looks = sourceOptions(fixture).map(sourceLook);
    options().forEach((o, i) => {
      const icon = o.querySelector<HTMLElement>('[data-test="client-source-icon"]')!;
      expect(icon.querySelector("path")!.getAttribute("d")).toBe(iconPathFor(looks[i]!.icon));
      expect(o.style.getPropertyValue("--k-c")).toBe(looks[i]!.color);
      expect(o.style.getPropertyValue("--k-on")).toBe("#fff");
      expect(o.classList.contains("client-source-option--picked")).toBe(false);
    });
    option(2).click();
    await flushPromises();
    expect(option(2).getAttribute("aria-checked")).toBe("true");
    expect(option(2).classList.contains("client-source-option--picked")).toBe(true);
    expect(option(2).style.getPropertyValue("--source-ring")).toBe(looks[2]!.color);
    expect(options().filter((o) => o.classList.contains("client-source-option--picked"))).toHaveLength(1);
  });

  it("one form: the keypad on the left, the name, keyboard and options on the right", async () => {
    await openForm(fixture);
    const form = q(".client-form")!;
    const [left, right] = Array.from(form.children) as HTMLElement[];
    expect(left!.classList.contains("client-form-phone")).toBe(true);
    expect(left!.querySelector(".client-phone-numpad-display-input")).not.toBeNull();
    expect(left!.querySelector(".client-phone-pad")).not.toBeNull();
    expect(left!.querySelector('[data-test="client-source-option"]')).toBeNull();
    expect(right!.classList.contains("client-form-details")).toBe(true);
    expect(Array.from(right!.querySelectorAll("input")).some((i) => i.placeholder === "Name")).toBe(true);
    expect(right!.querySelector(".client-name-keyboard")).not.toBeNull();
    expect(right!.querySelector('[role="radiogroup"]')!.querySelectorAll('[data-test="client-source-option"]')).toHaveLength(fixture.length);
    expect(right!.querySelector('[data-test="client-save"]')).not.toBeNull();
    expect(right!.textContent).toContain("Prefer not to say");
    // No separate step: the old phone screen's title and country cards are gone.
    expect(document.body.textContent).not.toContain("Enter Phone Number");
    expect(q(".client-phone-country-pager")).toBeNull();
  });

  it("name and answer can be filled before the number; save waits for a whole number", async () => {
    vi.mocked(GetClientByPhone).mockRejectedValue(new Error("not found"));
    await openForm(fixture);
    await typeName("rana");
    option(1).click();
    await flushPromises();
    expect(save().disabled).toBe(true);
    await typePhone("704567");
    expect(GetClientByPhone).not.toHaveBeenCalled();
    expect(save().disabled).toBe(true);
    await typePhone("70456781");
    expect(GetClientByPhone).toHaveBeenCalledWith("+96170456781");
    expect(save().disabled).toBe(false);
    save().click();
    await flushPromises();
    expect(CreateClient).toHaveBeenCalledWith("Rana", "+96170456781", fixture[1]!.uid);
  });

  it("a known number shows the client and Continue picks them, without creating anyone", async () => {
    vi.mocked(GetClientByPhone).mockResolvedValue({ id: 4, name: "Karim Saad", phone: "+96170456789" } as never);
    await openForm(fixture);
    await typePhone("70456789");
    expect(q('[data-test="client-found"]')!.textContent).toContain("Karim Saad");
    expect(options()).toHaveLength(0);
    expect(save().textContent?.trim()).toBe("Continue");
    expect(save().disabled).toBe(false);
    save().click();
    await flushPromises();
    expect(CreateClient).not.toHaveBeenCalled();
    expect(wrapper!.emitted("select")?.[0]?.[0]).toMatchObject({ id: 4, name: "Karim Saad" });
    expect(wrapper!.emitted("update:modelValue")?.at(-1)).toEqual([false]);
  });

  it("Enter on a known number picks the client", async () => {
    vi.mocked(GetClientByPhone).mockResolvedValue({ id: 4, name: "Karim Saad", phone: "+96170456789" } as never);
    await openForm(fixture);
    await typePhone("70456789");
    (q(".client-phone-numpad-display-input") as HTMLInputElement).dispatchEvent(new KeyboardEvent("keydown", { key: "Enter" }));
    await flushPromises();
    expect(wrapper!.emitted("select")?.[0]?.[0]).toMatchObject({ id: 4 });
  });

  it("changing a known number back to an unknown one brings the name and options back", async () => {
    vi.mocked(GetClientByPhone).mockImplementation(async (p: string) => {
      if (p === "+96170456789") return { id: 4, name: "Karim Saad", phone: p } as never;
      throw new Error("not found");
    });
    await openForm(fixture);
    await typePhone("70456789");
    expect(q('[data-test="client-found"]')).not.toBeNull();
    await typePhone("70456788");
    expect(q('[data-test="client-found"]')).toBeNull();
    expect(options()).toHaveLength(fixture.length);
    expect(save().textContent?.trim()).toBe("Save");
    expect(save().disabled).toBe(true);
  });

  it("a slow answer for an older number doesn't overwrite the newer one", async () => {
    let releaseOld: (v: unknown) => void = () => {};
    vi.mocked(GetClientByPhone).mockImplementation((p: string) => {
      if (p === "+96170456789") return new Promise((r) => (releaseOld = r)) as never;
      return Promise.reject(new Error("not found")) as never;
    });
    await openForm(fixture);
    await typePhone("70456789");
    await typePhone("70456788");
    releaseOld({ id: 4, name: "Karim Saad", phone: "+96170456789" });
    await flushPromises();
    expect(q('[data-test="client-found"]')).toBeNull();
    expect(save().textContent?.trim()).toBe("Save");
  });

  it("Prefer not to say picks the anonymous client", async () => {
    await openForm(fixture);
    const btn = qa("button").find((b) => b.textContent?.trim() === "Prefer not to say")!;
    btn.click();
    await flushPromises();
    expect(wrapper!.emitted("select")).toHaveLength(1);
    expect(CreateClient).not.toHaveBeenCalled();
  });

  it("with no options from central it doesn't ask, and saves with the name alone", async () => {
    await newCustomerStep([]);
    expect(document.body.textContent).not.toContain("Where did they hear about us?");
    await typeName("Omar");
    expect(save().disabled).toBe(false);
    save().click();
    await flushPromises();
    expect(CreateClient).toHaveBeenCalledWith("Omar", "+96170456789", "");
  });

  it("if the options can't be loaded, the customer can still be added", async () => {
    vi.mocked(GetClientByPhone).mockRejectedValue(new Error("not found"));
    await newCustomerStep([]);
    vi.mocked(GetClientSources).mockRejectedValue(new Error("db"));
    await typeName("Nour");
    save().click();
    await flushPromises();
    expect(CreateClient).toHaveBeenCalledWith("Nour", "+96170456789", "");
  });
});
