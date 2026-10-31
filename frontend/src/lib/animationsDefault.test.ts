import { describe, expect, it } from "vitest";
import { ANIMATIONS_KEY, initialAnimationsEnabled } from "./animationsDefault";

function memoryStore(initial: Record<string, string> = {}) {
  const data = { ...initial };
  return {
    data,
    getItem: (key: string): string | null => (key in data ? data[key]! : null),
    setItem: (key: string, value: string) => {
      data[key] = value;
    },
  };
}

describe("initialAnimationsEnabled", () => {
  it("is off on a fresh till", () => {
    expect(initialAnimationsEnabled(memoryStore())).toBe(false);
  });

  it("switches off a till that had the old default of on, once", () => {
    const store = memoryStore({ [ANIMATIONS_KEY]: "true" });
    expect(initialAnimationsEnabled(store)).toBe(false);
    expect(store.data[ANIMATIONS_KEY]).toBe("false");
  });

  it("keeps an explicit switch-on made after the reset", () => {
    const store = memoryStore();
    initialAnimationsEnabled(store);
    store.setItem(ANIMATIONS_KEY, "true");
    expect(initialAnimationsEnabled(store)).toBe(true);
  });

  it("stays off when nothing was chosen after the reset", () => {
    const store = memoryStore();
    initialAnimationsEnabled(store);
    expect(initialAnimationsEnabled(store)).toBe(false);
  });
});
