// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../wailsjs/go/main/App", () => ({
  GetDisplayPrefs: vi.fn(),
  ReportDisplayPrefs: vi.fn(() => Promise.resolve()),
}));
vi.mock("../../wailsjs/runtime/runtime", () => ({ EventsOn: vi.fn() }));

import { GetDisplayPrefs, ReportDisplayPrefs } from "../../wailsjs/go/main/App";

async function freshDisplay() {
  vi.resetModules();
  return import("./display");
}

// An in-memory localStorage: Node's own global shadows happy-dom's.
const stored = new Map<string, string>();
vi.stubGlobal("localStorage", {
  getItem: (key: string) => stored.get(key) ?? null,
  setItem: (key: string, value: string) => void stored.set(key, String(value)),
  removeItem: (key: string) => void stored.delete(key),
  clear: () => stored.clear(),
});

beforeEach(() => {
  localStorage.clear();
  vi.mocked(ReportDisplayPrefs).mockClear();
});

describe("Print return receipts", () => {
  it("is on until turned off", async () => {
    const display = await freshDisplay();
    expect(display.printReturnReceipts.value).toBe(true);
  });

  it("turning it off sticks and tells the till's backend", async () => {
    const display = await freshDisplay();
    display.setPrintReturnReceipts(false);
    expect(localStorage.getItem("kashi-print-return-receipts")).toBe("false");
    const reported = JSON.parse(vi.mocked(ReportDisplayPrefs).mock.calls.at(-1)![0] as string);
    expect(reported.print_return_receipts).toBe(false);
    expect((await freshDisplay()).printReturnReceipts.value).toBe(false);
  });

  it("follows a push from central", async () => {
    const display = await freshDisplay();
    vi.mocked(GetDisplayPrefs).mockResolvedValue({ prefs: { print_return_receipts: false }, rev: 5 } as never);
    await display.startDisplayPrefsSync(() => {});
    expect(display.printReturnReceipts.value).toBe(false);
    expect(localStorage.getItem("kashi-print-return-receipts")).toBe("false");
  });
});
