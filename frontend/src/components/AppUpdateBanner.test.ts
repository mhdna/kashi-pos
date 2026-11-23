// @vitest-environment happy-dom
//
// What the till says on its first start after an update. The "updated to … /
// What's new" dialog is off for now; the updater must still be told the new
// version started (or it rolls back), and a rollback or a failed update is
// still said.
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

vi.mock("../../wailsjs/go/main/App", () => ({
  ConfirmAppStarted: vi.fn(),
  GetAppUpdateStatus: vi.fn(async () => ({})),
  InstallAppUpdate: vi.fn(),
  WaitUntilReady: vi.fn(async () => ({})),
}));
vi.mock("../../wailsjs/runtime/runtime", () => ({ EventsOn: vi.fn(() => () => {}) }));

import { ConfirmAppStarted, WaitUntilReady } from "../../wailsjs/go/main/App";
import AppUpdateBanner from "./AppUpdateBanner.vue";

let wrapper: VueWrapper<any> | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  vi.clearAllMocks();
  document.body.innerHTML = "";
});

async function start(result: Record<string, any>) {
  vi.mocked(ConfirmAppStarted).mockResolvedValue(result as any);
  wrapper = mount(AppUpdateBanner, { global: { plugins: [ui] }, attachTo: document.body });
  await flushPromises();
  return document.body.textContent ?? "";
}

describe("AppUpdateBanner on start", () => {
  it("shows no What's new dialog after an update, and still confirms the start", async () => {
    const text = await start({
      result: "update",
      from: "1.1.105",
      to: "1.1.106",
      notes: "New: exchanges can be held",
    });
    expect(ConfirmAppStarted).toHaveBeenCalledTimes(1);
    expect(text).not.toContain("was updated to");
    expect(text).not.toContain("What's new");
    expect(text).not.toContain("exchanges can be held");
  });

  it("still says when it went back to the previous version", async () => {
    const text = await start({ result: "rollback", to: "1.1.105" });
    expect(text).toContain("GB Cloud went back to 1.1.105");
  });

  it("still says when an update didn't work", async () => {
    const text = await start({ result: "failed", from: "1.1.105", to: "1.1.106" });
    expect(text).toContain("The update didn't work");
    expect(text).toContain(
      "1.1.106 didn't start, so GB Cloud went back to the previous version. Your data is safe.",
    );
  });

  it("says nothing on an ordinary start", async () => {
    const text = await start({ result: "" });
    expect(text).not.toContain("GB Cloud");
  });

  it("doesn't confirm a start whose database didn't open", async () => {
    vi.mocked(WaitUntilReady).mockResolvedValueOnce({ error: "database locked" } as any);
    await start({ result: "update", to: "1.1.106" });
    expect(ConfirmAppStarted).not.toHaveBeenCalled();
  });
});

describe("the Updating GB Cloud screen", () => {
  it("is a plain spinner with no icon in the middle", async () => {
    const { readFileSync } = await import("node:fs");
    const { resolve } = await import("node:path");
    const src = readFileSync(resolve(__dirname, "AppUpdateBanner.vue"), "utf8");
    const block = src.slice(src.indexOf("<k-progress-circular indeterminate"), src.indexOf("Updating GB Cloud"));
    expect(block).toMatch(/<k-progress-circular[^>]*\/>/);
    expect(block).not.toMatch(/<k-icon/);
  });
});

// A forced update installs with no dialog, countdown, snackbar or "Updating…"
// screen; a normal one keeps them.
describe("AppUpdateBanner with a forced update", () => {
  async function mountWithEvents() {
    const handlers: Record<string, (s?: any) => void> = {};
    const { EventsOn } = await import("../../wailsjs/runtime/runtime");
    vi.mocked(EventsOn).mockImplementation(((name: string, fn: any) => {
      handlers[name] = fn;
      return () => {};
    }) as any);
    vi.mocked(ConfirmAppStarted).mockResolvedValue({ result: "" } as any);
    wrapper = mount(AppUpdateBanner, { global: { plugins: [ui] }, attachTo: document.body });
    await flushPromises();
    return handlers;
  }

  it("shows no 'ready' snackbar and no 'Updating' screen when forced", async () => {
    const h = await mountWithEvents();
    const forced = { state: "ready", version: "1.2.0", supported: true, force: true };
    h.app_update_state(forced);
    h.app_update_installing(forced);
    await flushPromises();
    const text = document.body.textContent ?? "";
    expect(text).not.toContain("is ready");
    expect(text).not.toContain("Updating GB Cloud");
    expect(text).not.toContain("required update");
  });

  it("still shows the snackbar and the 'Updating' screen for a normal update", async () => {
    const h = await mountWithEvents();
    const normal = { state: "ready", version: "1.2.0", supported: true, force: false };
    h.app_update_state(normal);
    await flushPromises();
    expect(document.body.textContent).toContain("GB Cloud 1.2.0 is ready");
    h.app_update_installing(normal);
    await flushPromises();
    expect(document.body.textContent).toContain("Updating GB Cloud");
  });
});
