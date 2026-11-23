// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CONTROLLED_MS, createLiveView, FLUSH_MS, framePacket, liveViewState, SNAPSHOT_MIN_MS, clickLabel } from "./liveView";

function rig() {
  let clock = 1_000_000;
  const timers: { fn: () => void; ms: number; id: number }[] = [];
  let timerId = 0;
  const sent: string[] = [];
  const mirror = { getNode: vi.fn((...[]: [number]) => null as Node | null) };
  let emit: (e: any) => void = () => {};
  const stopRecording = vi.fn();
  const takeFullSnapshot = vi.fn();
  const record: any = vi.fn((opts: any) => {
    emit = opts.emit;
    return stopRecording;
  });
  record.takeFullSnapshot = takeFullSnapshot;
  record.mirror = mirror;
  const loadRrweb = vi.fn(async () => ({ record }));
  let sendResult = true;
  const send = vi.fn(async (b: string) => {
    sent.push(b);
    return sendResult;
  });
  const live = createLiveView({
    loadRrweb,
    send,
    setTimer: (fn, ms) => {
      timers.push({ fn, ms, id: ++timerId });
      return timerId;
    },
    clearTimer: (id) => {
      const i = timers.findIndex((t) => t.id === id);
      if (i >= 0) timers.splice(i, 1);
    },
    now: () => clock,
  });
  return {
    live, loadRrweb, record, send, sent, stopRecording, takeFullSnapshot, mirror, timers,
    emit: (e: any) => emit(e),
    runTimers: () => timers.splice(0).forEach((t) => t.fn()),
    advance: (ms: number) => (clock += ms),
    setSendResult: (v: boolean) => (sendResult = v),
  };
}

beforeEach(() => {
  liveViewState.active = false;
  liveViewState.control = false;
  liveViewState.controlledUntil = 0;
});

describe("framePacket", () => {
  it("puts full first, which is what kashi looks for", () => {
    expect(framePacket([{ type: 2 }], [], true).startsWith('{"full":true')).toBe(true);
    expect(framePacket([{ type: 3 }], [], false).startsWith('{"e":')).toBe(true);
    expect(JSON.parse(framePacket([1], [{ k: "click", l: "Pay", at: 5 }], false))).toEqual({ e: [1], a: [{ k: "click", l: "Pay", at: 5 }] });
  });
});

describe("clickLabel", () => {
  it("names the button, not the span inside it, trimmed and short", () => {
    document.body.innerHTML = `<button><span id="s">  Pay   now </span></button><button aria-label="Close"><i id="x"></i></button>`;
    expect(clickLabel(document.getElementById("s"))).toBe("Pay now");
    expect(clickLabel(document.getElementById("x"))).toBe("Close");
    expect(clickLabel(null)).toBe("");
    document.body.innerHTML = `<button id="l">${"long ".repeat(30)}</button>`;
    expect(clickLabel(document.getElementById("l")).length).toBe(40);
  });
});

describe("live view recorder", () => {
  it("loads nothing and records nothing until started", () => {
    const r = rig();
    expect(r.loadRrweb).not.toHaveBeenCalled();
    expect(r.record).not.toHaveBeenCalled();
    expect(liveViewState.active).toBe(false);
  });

  it("starts recording with the safe options: passwords masked, pictures blocked", async () => {
    const r = rig();
    await r.live.start(false);
    expect(r.loadRrweb).toHaveBeenCalledTimes(1);
    const opts = r.record.mock.calls[0][0];
    expect(opts.maskInputOptions.password).toBe(true);
    expect(opts.blockSelector).toContain("img");
    expect(opts.recordCanvas).toBe(false);
    expect(opts.checkoutEveryNms).toBeGreaterThan(0);
    expect(liveViewState.active).toBe(true);
  });

  it("batches events into one packet per flush, the snapshot flagged full", async () => {
    const r = rig();
    await r.live.start(false);
    r.emit({ type: 4 });
    r.emit({ type: 2 });
    r.emit({ type: 3 });
    expect(r.timers).toHaveLength(1);
    expect(r.timers[0]!.ms).toBe(FLUSH_MS);
    expect(r.send).not.toHaveBeenCalled();
    r.runTimers();
    expect(r.sent).toHaveLength(1);
    expect(JSON.parse(r.sent[0]!).full).toBe(true);
    expect(JSON.parse(r.sent[0]!).e.map((e: any) => e.type)).toEqual([4, 2, 3]);

    r.emit({ type: 3 });
    r.runTimers();
    expect(r.sent).toHaveLength(2);
    expect(JSON.parse(r.sent[1]!).full).toBeUndefined();
  });

  it("sends nothing when nothing happened, and a click becomes an action", async () => {
    const r = rig();
    await r.live.start(false);
    r.runTimers();
    expect(r.send).not.toHaveBeenCalled();

    document.body.innerHTML = '<button id="b">Pay</button>';
    document.getElementById("b")!.click();
    r.runTimers();
    expect(JSON.parse(r.sent[0]!).a).toEqual([{ k: "click", l: "Pay", at: expect.any(Number) }]);
  });

  it("a dropped batch asks for a fresh snapshot, but never more often than every couple of seconds", async () => {
    const r = rig();
    await r.live.start(false);
    r.setSendResult(false);
    r.advance(SNAPSHOT_MIN_MS + 1);
    r.emit({ type: 3 });
    r.runTimers();
    await Promise.resolve();
    await Promise.resolve();
    expect(r.takeFullSnapshot).toHaveBeenCalledTimes(1);
    r.emit({ type: 3 });
    r.runTimers();
    await Promise.resolve();
    await Promise.resolve();
    expect(r.takeFullSnapshot).toHaveBeenCalledTimes(1); // too soon
    r.advance(SNAPSHOT_MIN_MS + 1);
    r.live.snapshot();
    expect(r.takeFullSnapshot).toHaveBeenCalledTimes(2);
  });

  it("kashi asking for a snapshot takes one; asking while stopped does nothing", async () => {
    const r = rig();
    r.live.snapshot();
    await r.live.start(false);
    r.advance(SNAPSHOT_MIN_MS + 1);
    r.live.snapshot();
    expect(r.takeFullSnapshot).toHaveBeenCalledTimes(1);
    r.live.stop();
    r.advance(SNAPSHOT_MIN_MS + 1);
    r.live.snapshot();
    expect(r.takeFullSnapshot).toHaveBeenCalledTimes(1);
  });

  it("stop ends the recording, cancels the flush and sends nothing after", async () => {
    const r = rig();
    await r.live.start(true);
    r.emit({ type: 3 });
    r.live.stop();
    expect(r.stopRecording).toHaveBeenCalled();
    expect(r.timers).toHaveLength(0);
    r.runTimers();
    expect(r.send).not.toHaveBeenCalled();
    expect(liveViewState).toMatchObject({ active: false, control: false, controlledUntil: 0 });
    // Clicks are no longer collected.
    document.body.innerHTML = '<button id="b">x</button>';
    document.getElementById("b")!.click();
    expect(r.timers).toHaveLength(0);
  });

  it("starting twice records once; rrweb is loaded once across sessions", async () => {
    const r = rig();
    await r.live.start(false);
    await r.live.start(true);
    expect(r.record).toHaveBeenCalledTimes(1);
    expect(liveViewState.control).toBe(true);
    r.live.stop();
    await r.live.start(false);
    expect(r.loadRrweb).toHaveBeenCalledTimes(1);
    expect(r.record).toHaveBeenCalledTimes(2);
  });

  it("a stop that lands while rrweb is still loading means no recording starts", async () => {
    const r = rig();
    let release!: () => void;
    r.loadRrweb.mockImplementationOnce(
      () => new Promise((resolve) => (release = () => resolve({ record: r.record }))) as any,
    );
    const starting = r.live.start(false);
    r.live.stop();
    release();
    await starting;
    expect(r.record).not.toHaveBeenCalled();
    expect(liveViewState.active).toBe(false);
  });

  describe("control", () => {
    it("applies a watcher's click only while control is allowed, and marks the screen as controlled", async () => {
      const r = rig();
      document.body.innerHTML = '<button id="b">Pay</button>';
      const button = document.getElementById("b")!;
      const onClick = vi.fn();
      button.addEventListener("click", onClick);
      r.mirror.getNode.mockReturnValue(button);

      await r.live.start(false);
      r.live.control('{"t":"click","id":3}');
      expect(onClick).not.toHaveBeenCalled();
      expect(liveViewState.controlledUntil).toBe(0);

      r.live.mode(true);
      r.live.control('{"t":"click","id":3}');
      expect(onClick).toHaveBeenCalledTimes(1);
      expect(liveViewState.controlledUntil).toBe(1_000_000 + CONTROLLED_MS);

      // Control taken away: dropped again, banner cleared at once.
      r.live.mode(false);
      r.live.control('{"t":"click","id":3}');
      expect(onClick).toHaveBeenCalledTimes(1);
      expect(liveViewState.controlledUntil).toBe(0);
    });

    it("ignores junk, unknown nodes, and anything while stopped", async () => {
      const r = rig();
      await r.live.start(true);
      r.mirror.getNode.mockReturnValue(null);
      r.live.control("not json");
      r.live.control('{"t":"eval"}');
      r.live.control('{"t":"click","id":9}');
      expect(liveViewState.controlledUntil).toBe(0);
      r.live.stop();
      r.live.control('{"t":"key","key":"Enter"}');
      expect(liveViewState.controlledUntil).toBe(0);
    });

    it("mode can not switch control on for a session that is not running", () => {
      const r = rig();
      r.live.mode(true);
      expect(liveViewState.control).toBe(false);
    });
  });
});
