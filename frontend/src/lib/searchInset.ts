// Searching in place: the results lie over the sale screen's right pane, down to
// where the search keyboard starts. How far up from the pane's bottom that is
// comes from where the two really are on screen, not from the keyboard's own
// height: if the pane ends above the bottom of the screen, a keyboard-sized
// inset leaves the last rows of the results under the keyboard, out of reach.

/** Pixels the results must stop short of the pane's bottom edge (never negative). */
export function searchResultsInset(paneBottom: number, keyboardTop: number): number {
  if (!Number.isFinite(paneBottom) || !Number.isFinite(keyboardTop)) return 0;
  return Math.max(0, Math.round(paneBottom - keyboardTop));
}
