// Live view of this till: while an admin watches it in kashi, record this
// page with rrweb and hand the frames to Go (LiveViewSend), which streams them
// to kashi. Everything starts only when Go says "live_view:start", so with the
// feature off nothing here runs and rrweb is never loaded. Docs: docs/dev/live-view.md.
import { reactive } from "vue";
import { applyControl, parseControl, parseStamp, type NodeMirror } from "./liveViewControl";

export const FLUSH_MS = 100;
/** Right after a watcher action the till sends on the next frame, not after FLUSH_MS. */
export const CONTROL_FLUSH_MS = 16;
/** How long after a watcher action the quick sending lasts, to catch what the action changes. */
export const CONTROL_FAST_MS = 600;
/** Full snapshots are big: never ask for one more often than this. */
export const SNAPSHOT_MIN_MS = 2000;
export const CHECKOUT_MS = 5 * 60 * 1000;
/** How long the "being controlled" banner stays after the last watcher action. */
export const CONTROLLED_MS = 8000;
const FULL_SNAPSHOT = 2;

export interface LiveAction {
  k: "click";
  l: string;
  at: number;
}

/** What the banner shows. */
export const liveViewState = reactive({ active: false, control: false, controlledUntil: 0 });

/**
 * One packet for Go. "full" is the first key on purpose: kashi checks the
 * start of the text to tell a full snapshot from an increment without
 * parsing it (api/live_hub.go fullFramePrefix).
 */
export function framePacket(events: unknown[], actions: LiveAction[], full: boolean, ack: number | null = null): string {
  const packet: Record<string, unknown> = full ? { full: true, e: events, a: actions } : { e: events, a: actions };
  if (ack !== null) packet.ack = ack;
  return JSON.stringify(packet);
}

/** What a click was on, in a few words, for the watcher's activity list. */
export function clickLabel(target: Element | null): string {
  const el = target?.closest?.("button, a, [role=button], label, input, select, textarea, .v-btn, .k-btn") ?? target;
  if (!el) return "";
  const text = (el.getAttribute("aria-label") || (el as HTMLElement).innerText || el.textContent || "").replace(/\s+/g, " ").trim();
  return text.slice(0, 40);
}

export interface Recorder {
  record: ((opts: any) => (() => void) | undefined) & { takeFullSnapshot(isCheckout?: boolean): void; mirror: NodeMirror };
}

export interface LiveViewDeps {
  /** Loads rrweb; called only on the first start. */
  loadRrweb: () => Promise<Recorder>;
  /** LiveViewSend: false means the batch was dropped. */
  send: (batch: string) => Promise<boolean>;
  setTimer?: (fn: () => void, ms: number) => unknown;
  clearTimer?: (id: unknown) => void;
  now?: () => number;
  doc?: Document;
}

export function createLiveView(deps: LiveViewDeps) {
  const setTimer = deps.setTimer ?? ((fn, ms) => setTimeout(fn, ms));
  const clearTimer = deps.clearTimer ?? ((id) => clearTimeout(id as any));
  const now = deps.now ?? Date.now;
  const doc = deps.doc ?? document;

  let rrweb: Recorder | null = null;
  let stopRecording: (() => void) | undefined;
  let events: unknown[] = [];
  let actions: LiveAction[] = [];
  let full = false;
  let timer: unknown = null;
  let fastUntil = 0;
  /** The stamp of the last watcher action, sent back once in the next packet. */
  let pendingAck: number | null = null;
  let lastSnapshot = 0;
  let generation = 0;
  let removeClicks: (() => void) | null = null;

  function flush() {
    timer = null;
    if (!liveViewState.active || (events.length === 0 && actions.length === 0 && pendingAck === null)) return;
    const packet = framePacket(events, actions, full, pendingAck);
    pendingAck = null;
    events = [];
    actions = [];
    full = false;
    void deps.send(packet).then((ok) => {
      // Dropped (stalled stream): show the watcher a fresh page instead.
      if (!ok && liveViewState.active) snapshot();
    });
  }

  function schedule() {
    if (timer === null) timer = setTimer(flush, now() < fastUntil ? CONTROL_FLUSH_MS : FLUSH_MS);
  }

  /** A watcher just acted: send what changed on the next frame instead of waiting. */
  function speedUp() {
    fastUntil = now() + CONTROL_FAST_MS;
    if (timer !== null) clearTimer(timer);
    timer = setTimer(flush, CONTROL_FLUSH_MS);
  }

  function snapshot() {
    if (!rrweb || !liveViewState.active) return;
    if (now() - lastSnapshot < SNAPSHOT_MIN_MS) return;
    lastSnapshot = now();
    rrweb.record.takeFullSnapshot(true);
  }

  async function start(control: boolean) {
    liveViewState.control = control;
    if (liveViewState.active) return;
    liveViewState.active = true;
    const mine = ++generation;
    rrweb ??= await deps.loadRrweb();
    // Stopped while rrweb was loading.
    if (mine !== generation || !liveViewState.active) return;
    events = [];
    actions = [];
    full = false;
    lastSnapshot = now();
    stopRecording = rrweb.record({
      emit(event: { type: number }) {
        if (event.type === FULL_SNAPSHOT) full = true;
        events.push(event);
        schedule();
      },
      checkoutEveryNms: CHECKOUT_MS,
      maskInputOptions: { password: true },
      sampling: { mousemove: 50, scroll: 150, input: "last" },
      inlineStylesheet: true,
      // Pictures become same-size blanks: no photo bytes over the wire.
      blockSelector: "img, video, canvas",
      recordCanvas: false,
    });
    const onClick = (e: MouseEvent) => {
      actions.push({ k: "click", l: clickLabel(e.target as Element | null), at: now() });
      schedule();
    };
    doc.addEventListener("click", onClick, true);
    removeClicks = () => doc.removeEventListener("click", onClick, true);
  }

  function stop() {
    generation++;
    liveViewState.active = false;
    liveViewState.control = false;
    liveViewState.controlledUntil = 0;
    stopRecording?.();
    stopRecording = undefined;
    removeClicks?.();
    removeClicks = null;
    if (timer !== null) clearTimer(timer);
    timer = null;
    fastUntil = 0;
    pendingAck = null;
    events = [];
    actions = [];
    full = false;
  }

  function control(raw: string) {
    if (!liveViewState.active || !liveViewState.control || !rrweb) return;
    const message = parseControl(raw);
    if (message && applyControl(message, rrweb.record.mirror, doc)) {
      liveViewState.controlledUntil = now() + CONTROLLED_MS;
      pendingAck = parseStamp(raw);
      speedUp();
    }
  }

  function mode(controlAllowed: boolean) {
    liveViewState.control = controlAllowed && liveViewState.active;
    if (!liveViewState.control) liveViewState.controlledUntil = 0;
  }

  return { start, stop, snapshot, control, mode };
}
