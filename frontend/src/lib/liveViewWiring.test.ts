// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises } from "@vue/test-utils";

const handlers = new Map<string, (...a: any[]) => void>();
vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: (name: string, fn: (...a: any[]) => void) => handlers.set(name, fn),
}));
let status = { active: false, control: false };
const LiveViewSend = vi.fn(async (_b: string) => true);
vi.mock("../../wailsjs/go/main/App", () => ({
  LiveViewSend: (b: string) => LiveViewSend(b),
  LiveViewStatus: async () => status,
}));
let rrwebLoads = 0;
const record: any = vi.fn(() => () => {});
record.takeFullSnapshot = vi.fn();
record.mirror = { getNode: () => null };
vi.mock("rrweb", () => {
  rrwebLoads++;
  return { record };
});

const { startLiveViewWiring } = await import("./liveViewWiring");
const { liveViewState } = await import("./liveView");

beforeEach(() => {
  handlers.clear();
  liveViewState.active = false;
  liveViewState.control = false;
  record.mockClear();
  status = { active: false, control: false };
});

describe("live view wiring", () => {
  it("with no session, rrweb is never loaded and nothing records", async () => {
    startLiveViewWiring();
    await flushPromises();
    expect(rrwebLoads).toBe(0);
    expect(record).not.toHaveBeenCalled();
    expect([...handlers.keys()].sort()).toEqual([
      "live_view:control", "live_view:mode", "live_view:snapshot", "live_view:start", "live_view:stop",
    ]);
  });

  it("Go's start loads rrweb and records; stop ends it", async () => {
    const live = startLiveViewWiring();
    handlers.get("live_view:start")!({ control: true });
    await flushPromises();
    expect(rrwebLoads).toBe(1);
    expect(record).toHaveBeenCalledTimes(1);
    expect(liveViewState).toMatchObject({ active: true, control: true });
    handlers.get("live_view:mode")!({ control: false });
    expect(liveViewState.control).toBe(false);
    handlers.get("live_view:stop")!();
    expect(liveViewState.active).toBe(false);
    expect(live).toBeTruthy();
  });

  it("a page that reloads mid-session resumes it", async () => {
    status = { active: true, control: true };
    startLiveViewWiring();
    await flushPromises();
    expect(record).toHaveBeenCalledTimes(1);
    expect(liveViewState).toMatchObject({ active: true, control: true });
  });
});
