// The separate screen's copy of the till app (live view, "separate screen")
// runs in a hidden iframe of the till's window, loaded as index.html?virtual=1.
// This file is the first import in main.ts and, only in that copy, makes it
// safe:
//   - every Go call goes through one binding (LiveVirtualCall), so Go can give
//     the copy its own sign-in and refuse whole-till actions;
//   - Go events and the Wails runtime come from the parent window, but only the
//     calls the copy needs, and its listeners are removed when it closes;
//   - localStorage and sessionStorage get a prefix, so the cashier's sale
//     draft is never read or written by the copy.
// In the till's own window nothing here runs. Docs: docs/dev/live-view.md.

export const VIRTUAL_STORAGE_PREFIX = "virtual:";

/** Whether this page is the separate screen's copy: in a frame, asked for with ?virtual=1. */
export function detectVirtual(win: { location: { search: string }; parent: unknown }): boolean {
  return win.parent !== win && new URLSearchParams(win.location.search).get("virtual") === "1";
}

/** A Storage whose keys all live under a prefix, in a backing Storage. */
export function prefixedStorage(backing: Storage, prefix = VIRTUAL_STORAGE_PREFIX): Storage {
  const own = () => {
    const keys: string[] = [];
    for (let i = 0; i < backing.length; i++) {
      const k = backing.key(i);
      if (k?.startsWith(prefix)) keys.push(k);
    }
    return keys;
  };
  const storage = {
    get length() {
      return own().length;
    },
    key: (i: number) => {
      const k = own()[i];
      return k === undefined ? null : k.slice(prefix.length);
    },
    getItem: (k: string) => backing.getItem(prefix + k),
    setItem: (k: string, v: string) => backing.setItem(prefix + k, String(v)),
    removeItem: (k: string) => backing.removeItem(prefix + k),
    clear: () => own().forEach((k) => backing.removeItem(k)),
  };
  return storage as Storage;
}

type Cancel = () => void;
export interface VirtualBridge {
  /** Removes every Go event listener this copy added. */
  teardown(): void;
}

// Only these parts of the Wails runtime exist in the copy; anything else
// (quit, window, browser, clipboard) does nothing, so it cannot act on the till.
const RUNTIME_ALLOWED = new Set(["EventsOnMultiple", "EventsOff", "EventsEmit", "LogPrint", "LogTrace", "LogDebug", "LogInfo", "LogWarning", "LogError"]);

/** The window.go the copy sees: every App method is a call through LiveVirtualCall. */
export function goProxy(parentGo: any): any {
  const call = (method: string) => async (...args: unknown[]) => {
    const text: string = await parentGo.main.App.LiveVirtualCall(method, JSON.stringify(args));
    return text === "" ? undefined : JSON.parse(text);
  };
  const app = new Proxy({}, { get: (_t, name) => (typeof name === "string" ? call(name) : undefined) });
  return { main: { App: app } };
}

/** The window.runtime the copy sees, built on the parent's. */
export function runtimeProxy(parentRuntime: any, cancels: Cancel[]): any {
  return new Proxy(
    {},
    {
      get(_t, name) {
        if (typeof name !== "string") return undefined;
        if (!RUNTIME_ALLOWED.has(name)) return () => Promise.resolve();
        if (name === "EventsOnMultiple") {
          return (event: string, callback: (...a: any[]) => void, max: number) => {
            const cancel = parentRuntime.EventsOnMultiple(event, callback, max);
            if (typeof cancel === "function") cancels.push(cancel);
            return cancel;
          };
        }
        return (...a: unknown[]) => parentRuntime[name](...a);
      },
    },
  );
}

/** Installs the bridge on a window whose parent is the till's window. */
export function installVirtualBridge(win: any): VirtualBridge {
  const parent = win.parent;
  const cancels: Cancel[] = [];
  Object.defineProperty(win, "go", { value: goProxy(parent.go), configurable: true, writable: true });
  Object.defineProperty(win, "runtime", { value: runtimeProxy(parent.runtime, cancels), configurable: true, writable: true });
  Object.defineProperty(win, "localStorage", { value: prefixedStorage(win.localStorage), configurable: true });
  Object.defineProperty(win, "sessionStorage", { value: prefixedStorage(win.sessionStorage), configurable: true });
  const bridge: VirtualBridge = {
    teardown() {
      for (const c of cancels.splice(0)) c();
    },
  };
  // The till's window calls this before it removes the frame.
  Object.defineProperty(win, "__virtualTeardown", { value: bridge.teardown, configurable: true });
  return bridge;
}

export const isVirtual = detectVirtual(window);
export const virtualBridge: VirtualBridge | null = isVirtual ? installVirtualBridge(window) : null;
