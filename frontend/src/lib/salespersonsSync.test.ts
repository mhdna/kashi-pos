// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from "vitest";
import { defineComponent, h } from "vue";
import { mount } from "@vue/test-utils";

const handlers = new Map<string, () => void>();
const off = vi.fn();
vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: vi.fn((name: string, fn: () => void) => {
    handlers.set(name, fn);
    return () => {
      handlers.delete(name);
      off();
    };
  }),
}));

import { onSalespersonsSynced, SALESPERSONS_SYNCED } from "./salespersonsSync";

function mountWith(reload: () => unknown) {
  return mount(defineComponent({ setup: () => (onSalespersonsSynced(reload), () => h("div")) }));
}

describe("onSalespersonsSynced", () => {
  beforeEach(() => {
    handlers.clear();
    off.mockClear();
  });

  it("listens for the till's salesperson sync event", () => {
    expect(SALESPERSONS_SYNCED).toBe("sync_applied:salespersons");
    mountWith(() => {});
    expect(handlers.has("sync_applied:salespersons")).toBe(true);
  });

  it("reloads every time central's change lands", () => {
    const reload = vi.fn();
    mountWith(reload);
    expect(reload).not.toHaveBeenCalled();
    handlers.get(SALESPERSONS_SYNCED)!();
    handlers.get(SALESPERSONS_SYNCED)!();
    expect(reload).toHaveBeenCalledTimes(2);
  });

  it("stops listening once the screen is closed", () => {
    const reload = vi.fn();
    const w = mountWith(reload);
    w.unmount();
    expect(off).toHaveBeenCalledTimes(1);
    expect(handlers.has(SALESPERSONS_SYNCED)).toBe(false);
  });
});
