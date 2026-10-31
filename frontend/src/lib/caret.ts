/**
 * The caret position (0..text.length) nearest to x, where x is measured
 * from the start of the text and width(n) is how wide its first n
 * characters draw. Used to put the caret where a finger lands in an input.
 */
export function caretIndexAt(text: string, x: number, width: (n: number) => number): number {
  if (x <= 0 || !text) return 0;
  let low = 0;
  let high = text.length;
  // Widths only grow with n: find the first boundary at or past x.
  while (low < high) {
    const mid = (low + high) >> 1;
    if (width(mid) < x) low = mid + 1;
    else high = mid;
  }
  if (low === 0) return 0;
  // Then pick whichever boundary, that one or the one before, is closer.
  return x - width(low - 1) <= width(low) - x ? low - 1 : low;
}

/**
 * Replaces [start, end) of value with inserted, as typing into a selection
 * does. Returns the new value and where the caret goes after it.
 */
export function replaceRange(value: string, start: number, end: number, inserted: string) {
  const from = Math.max(0, Math.min(start, end, value.length));
  const to = Math.min(value.length, Math.max(start, end));
  return { value: value.slice(0, from) + inserted + value.slice(to), caret: from + inserted.length };
}
