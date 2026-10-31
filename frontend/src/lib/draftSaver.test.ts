import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createDraftSaver } from "./draftSaver";

function memoryStore() {
  const data = new Map<string, string>();
  return {
    data,
    setItem: (k: string, v: string) => void data.set(k, v),
    removeItem: (k: string) => void data.delete(k),
  };
}

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe("createDraftSaver", () => {
  it("coalesces a burst of changes into one write of the latest state", () => {
    const store = memoryStore();
    const setItem = vi.spyOn(store, "setItem");
    const saver = createDraftSaver("k", store, 400);
    let state = "a";
    for (const next of ["a", "b", "c"]) {
      state = next;
      saver.save(() => state);
      vi.advanceTimersByTime(100);
    }
    expect(setItem).not.toHaveBeenCalled();
    vi.advanceTimersByTime(400);
    expect(setItem).toHaveBeenCalledTimes(1);
    expect(store.data.get("k")).toBe("c");
  });

  it("builds the draft when the save runs, not when it was queued", () => {
    const store = memoryStore();
    const saver = createDraftSaver("k", store, 400);
    let state = "old";
    saver.save(() => state);
    state = "new";
    vi.advanceTimersByTime(400);
    expect(store.data.get("k")).toBe("new");
  });

  it("remove deletes at once and cancels a pending save", () => {
    const store = memoryStore();
    const saver = createDraftSaver("k", store, 400);
    store.data.set("k", "stale");
    saver.save(() => "cart");
    saver.remove();
    expect(store.data.has("k")).toBe(false);
    vi.advanceTimersByTime(1000);
    expect(store.data.has("k")).toBe(false);
  });

  it("cancel drops a pending save but leaves what is stored", () => {
    const store = memoryStore();
    const saver = createDraftSaver("k", store, 400);
    store.data.set("k", "kept");
    saver.save(() => "cart");
    saver.cancel();
    vi.advanceTimersByTime(1000);
    expect(store.data.get("k")).toBe("kept");
  });

  it("flush writes a waiting save immediately, once", () => {
    const store = memoryStore();
    const setItem = vi.spyOn(store, "setItem");
    const saver = createDraftSaver("k", store, 400);
    saver.save(() => "cart");
    saver.flush();
    expect(store.data.get("k")).toBe("cart");
    vi.advanceTimersByTime(1000);
    expect(setItem).toHaveBeenCalledTimes(1);
  });

  it("writes nothing when the builder says not to (settlement mode)", () => {
    const store = memoryStore();
    const saver = createDraftSaver("k", store, 400);
    saver.save(() => null);
    vi.advanceTimersByTime(1000);
    expect(store.data.has("k")).toBe(false);
  });

  it("flush with nothing pending does nothing", () => {
    const store = memoryStore();
    createDraftSaver("k", store, 400).flush();
    expect(store.data.size).toBe(0);
  });
});
