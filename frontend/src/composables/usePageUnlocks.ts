import { ref } from "vue";
import { ListPageUnlocks } from "../../wailsjs/go/main/App";

/** Kept in sync with the `lockablePages` list in app.go. */
export const lockablePages = ["clients", "inventory", "transfers", "attendance", "salespersons"] as const;

const pageUnlocks = ref<Record<string, boolean>>({});
let loaded = false;

async function loadPageUnlocks() {
  try {
    pageUnlocks.value = (await ListPageUnlocks()) as Record<string, boolean>;
  } catch {
    /* keep previous state on failure */
  }
}

/**
 * Shared, module-cached map of which lockable pages an admin has unlocked
 * for non-admin users (missing/false = locked). Used by NavDrawer to gate
 * navigation and by the settings page to expose the toggles - sharing this
 * ref means flipping a toggle in Settings updates the nav drawer immediately.
 */
export function usePageUnlocks() {
  if (!loaded) {
    loaded = true;
    void loadPageUnlocks();
  }
  return { pageUnlocks, refreshPageUnlocks: loadPageUnlocks };
}
