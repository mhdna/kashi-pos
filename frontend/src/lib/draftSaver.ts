/**
 * Saves the sale draft without blocking the screen on every key.
 *
 * Serialising a big cart and writing localStorage (both synchronous) on each
 * scan or quantity key was felt on slow tills. Saves are coalesced: the draft
 * is built and written once, DELAY_MS after the last change, from whatever the
 * state is *then*. Removing the draft is never deferred, and it cancels a
 * pending save, so a sale that has just been paid can't be written back and
 * restored later.
 */

export const DRAFT_SAVE_DELAY_MS = 400;

type Store = Pick<Storage, "setItem" | "removeItem">;

export function createDraftSaver(key: string, store: Store = localStorage, delayMs = DRAFT_SAVE_DELAY_MS) {
  let timer: ReturnType<typeof setTimeout> | undefined;
  let pending: (() => string | null) | null = null;

  function run() {
    clearTimeout(timer);
    timer = undefined;
    const build = pending;
    pending = null;
    if (!build) return;
    const json = build();
    if (json !== null) store.setItem(key, json);
  }

  return {
    /** `build` runs when the save happens; null means "don't write". */
    save(build: () => string | null) {
      pending = build;
      clearTimeout(timer);
      timer = setTimeout(run, delayMs);
    },
    /** Drops the draft now and any save still waiting. */
    remove() {
      clearTimeout(timer);
      timer = undefined;
      pending = null;
      store.removeItem(key);
    },
    /** Forgets a waiting save without touching what is stored. */
    cancel() {
      clearTimeout(timer);
      timer = undefined;
      pending = null;
    },
    /** Writes a waiting save at once (leaving the page, closing the app). */
    flush: run,
  };
}
