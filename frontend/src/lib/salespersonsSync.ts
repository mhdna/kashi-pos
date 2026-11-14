import { onBeforeUnmount, onMounted } from "vue";
import { EventsOn } from "../../wailsjs/runtime/runtime";

// Central pushes a salesperson change to the till within a second, and the
// till then emits this event (emitSyncApplied in sync_pull.go). A screen that
// lists salespersons reloads on it, so a rename, a new person or one switched
// off in central shows at once, without leaving the page.
export const SALESPERSONS_SYNCED = "sync_applied:salespersons";

export function onSalespersonsSynced(reload: () => unknown) {
  let off: (() => void) | undefined;
  onMounted(() => {
    try {
      off = EventsOn(SALESPERSONS_SYNCED, () => void reload());
    } catch {
      // No Wails runtime (tests, a plain browser): nothing to listen to.
    }
  });
  onBeforeUnmount(() => off?.());
}
