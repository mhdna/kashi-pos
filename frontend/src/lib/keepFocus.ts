/**
 * A mousedown handler that keeps the field being typed into focused, with
 * its caret and selection, when the press lands anywhere else: the browser
 * moves focus on mousedown, so cancelling it keeps the field's while the
 * click itself still happens. Other fields, and a scroller (a mouse on its
 * scrollbar, to drag it), are let through.
 */
export function keepFieldFocus(event: MouseEvent) {
  const target = event.target as Element;
  if (target.closest("input, textarea") || target.classList.contains("v-virtual-scroll")) return;
  event.preventDefault();
}
