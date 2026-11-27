import { readFileSync } from "node:fs";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { locale, setLocale } from "@/i18n";
import { startTillLanguage } from "./tillLanguage";

let language: string | undefined = "fr";
let fail = false;
let changed: () => void = () => {};
vi.mock("../../wailsjs/go/main/App", () => ({
  GetAppSettings: async () => {
    if (fail) throw new Error("no database");
    return { tillLanguage: language };
  },
}));
vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: (name: string, fn: () => void) => {
    if (name === "settings_changed") changed = fn;
    return () => {};
  },
}));
const settle = () => new Promise((r) => setTimeout(r));

describe("till language", () => {
  beforeEach(() => {
    fail = false;
  });

  it("shows the setting at start, over what the screen last showed", async () => {
    setLocale("en");
    language = "fr";
    startTillLanguage();
    await settle();
    expect(locale.value).toBe("fr");
  });

  it("follows every later change", async () => {
    startTillLanguage();
    for (const next of ["en", "fr", "fr", "en"]) {
      language = next;
      changed();
      await settle();
      expect(locale.value).toBe(next);
    }
  });

  it("keeps the current language when settings are missing or unreadable", async () => {
    setLocale("en");
    language = undefined;
    startTillLanguage();
    await settle();
    expect(locale.value).toBe("en");
    fail = true;
    changed();
    await settle();
    expect(locale.value).toBe("en");
  });
});

describe("till language wiring", () => {
  const read = (p: string) => readFileSync(`${process.cwd()}/src/${p}`, "utf8");

  it("is started with the app", () => {
    expect(read("App.vue")).toMatch(/^\s*startTillLanguage\(\);/m);
  });

  it("can be changed from Settings, each language named in itself", () => {
    const settings = read("pages/settings.vue");
    expect(settings).toContain('data-test="till-language"');
    expect(settings).toContain('@update:model-value="saveTillLanguage"');
    expect(settings).toContain('{ title: "Français", value: "fr" }');
    expect(settings).toContain('{ title: "English", value: "en" }');
    expect(settings).toContain("await SetTillLanguage(language)");
  });
});
