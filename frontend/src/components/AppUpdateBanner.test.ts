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
