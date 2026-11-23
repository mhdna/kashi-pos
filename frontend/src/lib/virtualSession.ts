// The separate screen, in the till's own window: a hidden iframe holding a
// second copy of the till app (index.html?virtual=1, see virtualBridge.ts) that
// an admin drives from kashi while the cashier keeps selling. Go says when to
// open and close it ("live_virtual:start" / "live_virtual:stop"), and only
// while kashi has live view, control and a user who may use it.
//
// The copy sits off screen at the window's size (not display:none, layout needs
// a size). The cashier's keyboard and barcode scanner must never land in it, so
// whenever focus moves into the frame it is handed straight back.
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { LiveVirtualStatus } from "../../wailsjs/go/main/App";

export const VIRTUAL_SRC = "index.html?virtual=1#/";

/** Puts focus back where it was whenever it moves into the frame. Returns the remover. */
export function installFocusGuard(doc: Document, win: Window, frame: HTMLIFrameElement): () => void {
  let last: HTMLElement | null = null;
  const onFocusIn = (e: Event) => {
    const t = e.target as HTMLElement | null;
    if (t && t !== (frame as unknown) && typeof t.focus === "function") last = t;
  };
  const restore = () => {
    if (doc.activeElement !== frame) return;
    frame.blur();
    if (last?.isConnected) last.focus();
  };
  const onFrameFocus = () => restore();
  doc.addEventListener("focusin", onFocusIn, true);
  win.addEventListener("blur", restore);
  let inner: Window | null = null;
  const onLoad = () => {
    inner = frame.contentWindow;
    inner?.addEventListener("focus", onFrameFocus, true);
  };
  frame.addEventListener("load", onLoad);
  return () => {
    doc.removeEventListener("focusin", onFocusIn, true);
    win.removeEventListener("blur", restore);
    frame.removeEventListener("load", onLoad);
    inner?.removeEventListener("focus", onFrameFocus, true);
  };
}

export function createVirtualSession(deps: { doc?: Document; win?: Window; src?: string } = {}) {
  const doc = deps.doc ?? document;
  const win = deps.win ?? window;
  const src = deps.src ?? VIRTUAL_SRC;
  let frame: HTMLIFrameElement | null = null;
  let removeGuard: (() => void) | null = null;

  function open() {
    if (frame) return;
    const f = doc.createElement("iframe");
    f.setAttribute("tabindex", "-1");
    f.setAttribute("aria-hidden", "true");
    f.setAttribute("data-virtual-screen", "");
    f.style.cssText = `position:fixed;left:-10000px;top:0;width:${win.innerWidth}px;height:${win.innerHeight}px;border:0;pointer-events:none;`;
    removeGuard = installFocusGuard(doc, win, f);
    f.src = src;
    doc.body.append(f);
    frame = f;
  }

  function close() {
    if (!frame) return;
    // The copy's Go event listeners live in this window's runtime: drop them first.
    try {
      (frame.contentWindow as any)?.__virtualTeardown?.();
    } catch {
      // A frame that is already gone has nothing to tear down.
    }
    removeGuard?.();
    removeGuard = null;
    frame.remove();
    frame = null;
  }

  return { open, close, isOpen: () => frame !== null };
}

/** Called once from App.vue in the till's own window. */
export function startVirtualSessionWiring() {
  const session = createVirtualSession();
  EventsOn("live_virtual:start", () => session.open());
  EventsOn("live_virtual:stop", () => session.close());
  // A reloaded till window picks up a separate screen that is already open.
  void LiveVirtualStatus()
    .then((s) => {
      if (s?.active) session.open();
    })
    .catch(() => undefined);
  return session;
}
