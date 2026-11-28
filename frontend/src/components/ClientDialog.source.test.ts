// @vitest-environment happy-dom
// The new-customer step asks where they heard about us: the options are the
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

async function newCustomerStep(sources: unknown) {
  vi.mocked(GetClientByPhone).mockRejectedValue(new Error("not found"));
  vi.mocked(GetClientSources).mockResolvedValue(sources as never);
  vi.mocked(CreateClient).mockImplementation(async (name: string, phone: string) => ({ id: 9, name, phone }) as never);
  wrapper = mount(ClientDialog, { props: { modelValue: true }, global: { plugins: [ui] }, attachTo: document.body });
  await flushPromises();
  const phone = q(".client-phone-numpad-display-input") as HTMLInputElement;
  phone.value = "70456789";
  phone.dispatchEvent(new Event("input"));
  phone.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter" }));
  await flushPromises();
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

  it("the options stand in a column on the right, beside the fields and keyboard", async () => {
    await newCustomerStep(fixture);
    const layout = q(".new-client-layout")!;
    const [main, side] = Array.from(layout.children) as HTMLElement[];
    expect(main!.classList.contains("new-client-main")).toBe(true);
    expect(main!.querySelector(".client-name-keyboard")).not.toBeNull();
    expect(main!.querySelector('[data-test="client-source-option"]')).toBeNull();
    expect(side!.getAttribute("role")).toBe("radiogroup");
    expect(side!.querySelectorAll('[data-test="client-source-option"]')).toHaveLength(fixture.length);
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

describe("ClientDialog: the new client's number pops up a keypad", () => {
  const phoneField = () =>
    qa("input").find((i) => (i as HTMLInputElement).placeholder === "Phone Number") as HTMLInputElement;
  const pad = () => q('[data-test="phone-pad-menu"]');
  const key = (k: string) => q(`[data-test="phone-pad-menu-key-${k}"]`)!;
  const tap = async (el: Element) => {
    el.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();
  };
  const letterKeyboard = () => q(".client-name-keyboard");

  it("the reported case: tapping the number opens a popup pad and the letter keyboard stays", async () => {
    await newCustomerStep(fixture);
    expect(letterKeyboard()).not.toBeNull();
    expect(pad()).toBeNull();
    await tap(phoneField());
    expect(pad()).not.toBeNull();
    expect(q('[data-test="phone-pad-menu"] .phone-pad-menu-title')!.textContent).toBe("Phone number");
    expect(letterKeyboard()).not.toBeNull();
    // No inline keypad takes the keyboard's place any more.
    expect(q(".new-client-main .phone-pad")).toBeNull();
  });

  it("the pad's keys edit the number, shown formatted, and Done closes it", async () => {
    await newCustomerStep(fixture);
    await tap(phoneField());
    expect(q('[data-test="phone-pad-menu-display"]')!.textContent).toBe("+961 (70) 456-789");
    await tap(key("back"));
    await tap(key("1"));
    expect(q('[data-test="phone-pad-menu-display"]')!.textContent).toBe("+961 (70) 456-781");
    await tap(key("clear"));
    expect(q('[data-test="phone-pad-menu-display"]')!.textContent).toBe("0");
    for (const d of "78255566") await tap(key(d));
    expect(phoneField().value).toBe("(78) 255-566");
    await tap(q('[data-test="phone-pad-menu-done"]')!);
    expect(pad()).toBeNull();

    await typeName("lina");
    option(0).click();
    await flushPromises();
    save().click();
    await flushPromises();
    expect(CreateClient).toHaveBeenCalledWith("Lina", "+96178255566", fixture[0]!.uid);
  });
});
