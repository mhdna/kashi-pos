// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises } from "@vue/test-utils";

const handlers = new Map<string, (...a: any[]) => void>();
vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: (name: string, fn: (...a: any[]) => void) => handlers.set(name, fn),
}));
let emit: (e: any) => void = () => {};
const record: any = vi.fn((opts: any) => {
  emit = opts.emit;
  return () => {};
});
record.takeFullSnapshot = vi.fn();
record.mirror = { getNode: () => null };
vi.mock("rrweb", () => ({ record }));

const { startVirtualRecorder } = await import("./virtualRecorder");
const { liveViewState } = await import("./liveView");

beforeEach(() => {
  handlers.clear();
  record.mockClear();
  liveViewState.active = false;
  liveViewState.control = false;
});

describe("the separate screen's recorder", () => {
  it("records at once with control on and sends frames to the till window's separate stream", async () => {
    vi.useFakeTimers();
    const LiveVirtualSend = vi.fn(async (_b: string) => true);
    const LiveViewSend = vi.fn();
    startVirtualRecorder({ main: { App: { LiveVirtualSend, LiveViewSend } } });
    await vi.advanceTimersByTimeAsync(0);
    expect(record).toHaveBeenCalledTimes(1);
    expect(liveViewState.control).toBe(true);
    emit({ type: 2, data: {} });
    await vi.advanceTimersByTimeAsync(200);
    expect(LiveVirtualSend).toHaveBeenCalledTimes(1);
    expect(LiveVirtualSend.mock.calls[0]![0]).toMatch(/^\{"full":true/);
    expect(LiveViewSend).not.toHaveBeenCalled(); // never the mirror's stream
    vi.useRealTimers();
  });

  it("listens to the separate screen's events only, never the mirror's", async () => {
    startVirtualRecorder({ main: { App: { LiveVirtualSend: async () => true } } });
    await flushPromises();
    expect([...handlers.keys()].sort()).toEqual(["live_virtual:control", "live_virtual:snapshot"]);
  });
});
