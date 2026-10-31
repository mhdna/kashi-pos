import { onBeforeUnmount, watch, type Ref } from "vue";

// Overlays (dialogs, menus, tooltips) stack: the newest gets the highest
// z-index and is the only one that reacts to Escape.
let zCounter = 2000;
const stack: Array<{ id: number; close: () => void }> = [];
let nextId = 1;
let listening = false;

export function nextZ(): number {
  return ++zCounter;
}

// Closes the newest overlay; returns false when none is open.
export function closeTopOverlay(): boolean {
  const top = stack[stack.length - 1];
  if (!top) return false;
  top.close();
  return true;
}

function onKeydown(event: KeyboardEvent) {
  if (event.key !== "Escape") return;
  if (closeTopOverlay()) event.stopPropagation();
}

export function pushOverlay(close: () => void): () => void {
  const entry = { id: nextId++, close };
  stack.push(entry);
  if (!listening && typeof document !== "undefined") {
    document.addEventListener("keydown", onKeydown, true);
    listening = true;
  }
  return () => {
    const i = stack.indexOf(entry);
    if (i >= 0) stack.splice(i, 1);
    if (stack.length === 0) zCounter = 2000;
    if (stack.length === 0 && listening && typeof document !== "undefined") {
      document.removeEventListener("keydown", onKeydown, true);
      listening = false;
    }
  };
}

export function overlayDepth(): number {
  return stack.length;
}

// While `open` is true, register `close` as the Escape target.
export function useEscapeToClose(open: Ref<boolean>, close: () => void, enabled: () => boolean = () => true) {
  let release: (() => void) | undefined;
  const stop = watch(
    open,
    (value) => {
      release?.();
      release = undefined;
      if (value) release = pushOverlay(() => enabled() && close());
    },
    { immediate: true },
  );
  onBeforeUnmount(() => {
    stop();
    release?.();
  });
}
