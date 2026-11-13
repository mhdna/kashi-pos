// The flip counter turns one card per digit. When the count changes, the digits
// that differ are always a run at the right-hand end (a carry or a borrow:
// 479 to 480 changes the last two). This finds that run, measured from the
// left of the NEW number, so one highlight can sit behind all of it.
//
// A card that is new (the number grew: 99 to 100) counts as changed; when the
// number shrinks (100 to 99) the cards that remain are compared with the
// digits that were in the same place from the right.

export function changedRun(before: number, now: number): { from: number; count: number } | null {
  const a = String(before);
  const b = String(now);
  if (a === b) return null;
  for (let i = 0; i < b.length; i++) {
    const fromRight = b.length - 1 - i;
    const old = a[a.length - 1 - fromRight];
    if (old !== b[i]) return { from: i, count: b.length - i };
  }
  // Same digits at every place, but the number is shorter (e.g. 0100 style
  // never happens for counts), so nothing on screen changed.
  return null;
}
