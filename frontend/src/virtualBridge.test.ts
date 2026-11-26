// @vitest-environment happy-dom
import { describe, expect, it, vi } from "vitest";
import { detectVirtual, goProxy, installVirtualBridge, prefixedStorage, runtimeProxy, VIRTUAL_STORAGE_PREFIX } from "./virtualBridge";

function memoryStorage(): Storage {
  const m = new Map<string, string>();
  return {
    get length() { return m.size; },
    key: (i: number) => [...m.keys()][i] ?? null,
    getItem: (k: string) => m.get(k) ?? null,
    setItem: (k: string, v: string) => void m.set(k, String(v)),
    removeItem: (k: string) => void m.delete(k),
    clear: () => m.clear(),
  } as Storage;
}

function fakeParent() {
  const cancels: Array<ReturnType<typeof vi.fn>> = [];
  const LiveVirtualCall = vi.fn(async (method: string, args: string) => JSON.stringify({ method, args: JSON.parse(args) }));
  const rawBinding = vi.fn();
  const runtime = {
    EventsOnMultiple: vi.fn((..._a: any[]) => { const c = vi.fn(); cancels.push(c); return c; }),
    EventsOff: vi.fn(), EventsEmit: vi.fn(), LogInfo: vi.fn(),
    Quit: vi.fn(), WindowReload: vi.fn(), BrowserOpenURL: vi.fn(),
  };
  return { go: { main: { App: { LiveVirtualCall, VerifyPIN: rawBinding, RestartApp: rawBinding } } }, runtime, cancels, LiveVirtualCall, rawBinding };
}

describe("detectVirtual", () => {
  it("is only the copy in a frame that asked for ?virtual=1", () => {
    const top: any = { location: { search: "?virtual=1" } };
    top.parent = top;
    expect(detectVirtual(top)).toBe(false); // the till's own window, even with the flag
    expect(detectVirtual({ location: { search: "?virtual=1" }, parent: {} })).toBe(true);
    expect(detectVirtual({ location: { search: "" }, parent: {} })).toBe(false);
    expect(detectVirtual({ location: { search: "?virtual=0" }, parent: {} })).toBe(false);
  });
});

describe("goProxy", () => {
  it("sends every App call through LiveVirtualCall and nothing else", async () => {
    const p = fakeParent();
    const go = goProxy(p.go);
    expect(await go.main.App.VerifyPIN("1478")).toEqual({ method: "VerifyPIN", args: ["1478"] });
    expect(await go.main.App.AnythingAtAll(1, { a: 2 }, [3])).toEqual({ method: "AnythingAtAll", args: [1, { a: 2 }, [3]] });
    expect(p.rawBinding).not.toHaveBeenCalled(); // the raw bindings are never reachable
    expect(Object.keys(go)).toEqual(["main"]);
  });

  it("turns an empty answer into undefined and passes errors on", async () => {
    const p = fakeParent();
    p.LiveVirtualCall.mockResolvedValueOnce("");
    expect(await goProxy(p.go).main.App.X()).toBeUndefined();
    p.LiveVirtualCall.mockRejectedValueOnce("Not available on the separate screen");
    await expect(goProxy(p.go).main.App.RestartApp()).rejects.toBe("Not available on the separate screen");
  });
});

describe("runtimeProxy", () => {
  it("shares events with the parent and removes the listeners on teardown", () => {
    const p = fakeParent();
    const cancels: Array<() => void> = [];
    const rt = runtimeProxy(p.runtime, cancels);
    const cb = () => {};
    rt.EventsOnMultiple("settings_changed", cb, -1);
    rt.EventsOnMultiple("sync_applied:x", cb, 1);
    expect(p.runtime.EventsOnMultiple).toHaveBeenCalledWith("settings_changed", cb, -1);
    expect(cancels).toHaveLength(2);
    rt.LogInfo("hi");
    expect(p.runtime.LogInfo).toHaveBeenCalledWith("hi");
  });

  it("does nothing for quit, reload, browser and any other runtime call", async () => {
    const p = fakeParent();
    const rt = runtimeProxy(p.runtime, []);
    await rt.Quit();
    await rt.WindowReload();
    await rt.BrowserOpenURL("https://x");
    await rt.SomethingNew();
    expect(p.runtime.Quit).not.toHaveBeenCalled();
    expect(p.runtime.WindowReload).not.toHaveBeenCalled();
    expect(p.runtime.BrowserOpenURL).not.toHaveBeenCalled();
  });
});

describe("prefixedStorage", () => {
  it("never reads or writes the cashier's keys", () => {
    const real = memoryStorage();
    real.setItem("kashi-pos.invoice-draft", "CASHIER CART");
    const s = prefixedStorage(real);
    expect(s.getItem("kashi-pos.invoice-draft")).toBeNull();
    s.setItem("kashi-pos.invoice-draft", "ADMIN");
    expect(real.getItem("kashi-pos.invoice-draft")).toBe("CASHIER CART");
    expect(real.getItem(VIRTUAL_STORAGE_PREFIX + "kashi-pos.invoice-draft")).toBe("ADMIN");
    expect(s.getItem("kashi-pos.invoice-draft")).toBe("ADMIN");
    s.removeItem("kashi-pos.invoice-draft");
    expect(real.getItem("kashi-pos.invoice-draft")).toBe("CASHIER CART");
  });

  it("clear and length only see the copy's own keys", () => {
    const real = memoryStorage();
    real.setItem("a", "1");
    const s = prefixedStorage(real);
    s.setItem("x", "1");
    s.setItem("y", "2");
    expect(s.length).toBe(2);
    expect([s.key(0), s.key(1), s.key(2)]).toEqual(["x", "y", null]);
    s.clear();
    expect(s.length).toBe(0);
    expect(real.getItem("a")).toBe("1");
  });
});

describe("installVirtualBridge", () => {
  it("swaps go, runtime and both storages on the iframe window, and teardown cancels events", () => {
    const p = fakeParent();
    const local = memoryStorage();
    local.setItem("kashi-pos.invoice-draft", "CASHIER CART");
    const win: any = { parent: { go: p.go, runtime: p.runtime }, localStorage: local, sessionStorage: memoryStorage() };
    const bridge = installVirtualBridge(win);
    win.localStorage.setItem("k", "v");
    expect(local.getItem("k")).toBeNull();
    expect(local.getItem("virtual:k")).toBe("v");
    expect(win.localStorage.getItem("kashi-pos.invoice-draft")).toBeNull();
    win.runtime.EventsOnMultiple("e", () => {}, -1);
    bridge.teardown();
    expect(p.cancels[0]).toHaveBeenCalledTimes(1);
    bridge.teardown();
    expect(p.cancels[0]).toHaveBeenCalledTimes(1);
    expect(typeof win.go.main.App.VerifyPIN).toBe("function");
  });
});
