/**
 * Whether the till's animations start on.
 *
 * They are off by default on every build: the moving parts cost repaints on
 * older machines. Before v1.1.82 the default was on, so most tills that
 * report "on" never chose it; the first start of this version therefore
 * switches every till off once. After that an explicit choice (Settings, or
 * a push from central) is kept.
 */

export const ANIMATIONS_KEY = "kashi-animations-enabled";
export const ANIMATIONS_RESET_KEY = "kashi-animations-reset-v1";

type Store = Pick<Storage, "getItem" | "setItem">;

export function initialAnimationsEnabled(store: Store): boolean {
  if (store.getItem(ANIMATIONS_RESET_KEY) === null) {
    store.setItem(ANIMATIONS_KEY, "false");
    store.setItem(ANIMATIONS_RESET_KEY, "1");
    return false;
  }
  return store.getItem(ANIMATIONS_KEY) === "true";
}
