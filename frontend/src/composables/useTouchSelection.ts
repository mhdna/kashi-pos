import { onBeforeUnmount, watch, type Ref } from "vue";
import { caretIndexAt } from "@/lib/caret";

const DRAG_START_PX = 6;
const DOUBLE_TAP_MS = 350;
const EDGE_PX = 24;

/**
 * Touch editing for a one-line input: a tap puts the caret under the
 * finger, a drag selects from where it went down to where it is (scrolling
 * the text along at either edge), and a double tap selects everything, so
 * the next key replaces the whole search.
 *
 * WebView2's own touch handling of an input mostly scrolls its text or does
 * nothing, so this takes the touch over: the input gets touch-action: none
 * and the pointerdown's default (which would also send the mouse events
 * that move the caret again) is cancelled. A mouse and a pen keep the
 * browser's own behaviour.
 */
export function useTouchSelection(input: Ref<HTMLInputElement | null>) {
  let anchor = -1;
  let pointer = -1;
  let downX = 0;
  let dragging = false;
  let lastTap = 0;
  let canvas: CanvasRenderingContext2D | null = null;

  function indexAt(el: HTMLInputElement, clientX: number) {
    const style = getComputedStyle(el);
    canvas ??= document.createElement("canvas").getContext("2d");
    if (!canvas) return el.value.length;
    canvas.font = `${style.fontStyle} ${style.fontWeight} ${style.fontSize} ${style.fontFamily}`;
    const spacing = Number.parseFloat(style.letterSpacing) || 0;
    const left = el.getBoundingClientRect().left + Number.parseFloat(style.borderLeftWidth) + Number.parseFloat(style.paddingLeft);
    const text = el.value;
    return caretIndexAt(text, clientX - left + el.scrollLeft, (n) => canvas!.measureText(text.slice(0, n)).width + n * spacing);
  }

  function onDown(event: PointerEvent) {
    const el = input.value;
    if (!el || event.pointerType !== "touch" || pointer !== -1) return;
    event.preventDefault();
    el.focus({ preventScroll: true });
    el.setPointerCapture?.(event.pointerId);
    pointer = event.pointerId;
    downX = event.clientX;
    dragging = false;
    anchor = indexAt(el, event.clientX);
    el.setSelectionRange(anchor, anchor);
  }

  function onMove(event: PointerEvent) {
    const el = input.value;
    if (!el || event.pointerId !== pointer) return;
    if (!dragging && Math.abs(event.clientX - downX) < DRAG_START_PX) return;
    dragging = true;
    // Past an edge, scroll the text so the selection can reach beyond what shows.
    const rect = el.getBoundingClientRect();
    if (event.clientX < rect.left + EDGE_PX) el.scrollLeft -= 8;
    else if (event.clientX > rect.right - EDGE_PX) el.scrollLeft += 8;
    const at = indexAt(el, event.clientX);
    el.setSelectionRange(Math.min(anchor, at), Math.max(anchor, at), at < anchor ? "backward" : "forward");
  }

  function onUp(event: PointerEvent) {
    const el = input.value;
    if (!el || event.pointerId !== pointer) return;
    pointer = -1;
    if (dragging) {
      lastTap = 0;
      return;
    }
    const now = event.timeStamp;
    if (now - lastTap < DOUBLE_TAP_MS) {
      el.select();
      lastTap = 0;
    } else {
      lastTap = now;
    }
  }

  function onCancel(event: PointerEvent) {
    if (event.pointerId === pointer) pointer = -1;
  }

  const listeners = [
    ["pointerdown", onDown],
    ["pointermove", onMove],
    ["pointerup", onUp],
    ["pointercancel", onCancel],
  ] as const;

  function attach(el: HTMLInputElement) {
    el.style.touchAction = "none";
    for (const [name, handler] of listeners) el.addEventListener(name, handler);
  }

  function detach(el: HTMLInputElement) {
    for (const [name, handler] of listeners) el.removeEventListener(name, handler);
  }

  watch(
    input,
    (el, old) => {
      if (old) detach(old);
      if (el) attach(el);
      pointer = -1;
    },
    { immediate: true },
  );
  onBeforeUnmount(() => {
    if (input.value) detach(input.value);
  });
}
