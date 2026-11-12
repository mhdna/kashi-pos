// The tab bar scrolls sideways when its tabs don't fit (Settings has more
// tabs than a till screen is wide). Kept as plain functions so the edge cases
// are tested without a layout engine.

/** Which edge arrows to show; 1px of slack hides rounding noise. */
export function arrowState(scrollLeft: number, clientWidth: number, scrollWidth: number): { prev: boolean; next: boolean } {
  const max = Math.max(0, scrollWidth - clientWidth);
  return { prev: scrollLeft > 1, next: scrollLeft < max - 1 };
}

/** Where an arrow tap goes: most of a screen-width, clamped to the ends. */
export function pageTarget(scrollLeft: number, clientWidth: number, scrollWidth: number, dir: -1 | 1): number {
  const max = Math.max(0, scrollWidth - clientWidth);
  const step = Math.max(80, Math.round(clientWidth * 0.8));
  return Math.min(max, Math.max(0, scrollLeft + dir * step));
}

/** A mouse wheel turns vertically; on a sideways strip that should scroll it. */
export function wheelToX(deltaX: number, deltaY: number): number {
  return Math.abs(deltaX) > Math.abs(deltaY) ? deltaX : deltaY;
}

/** Scroll needed so [left, left+width] is fully inside the view, with a margin. */
export function revealTarget(scrollLeft: number, clientWidth: number, scrollWidth: number, left: number, width: number, margin = 40): number {
  const max = Math.max(0, scrollWidth - clientWidth);
  let target = scrollLeft;
  if (left - margin < scrollLeft) target = left - margin;
  else if (left + width + margin > scrollLeft + clientWidth) target = left + width + margin - clientWidth;
  return Math.min(max, Math.max(0, target));
}
