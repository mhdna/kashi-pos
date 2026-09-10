import { ref } from "vue";
import { GetSyncStatus } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import type { main } from "../../wailsjs/go/models";

/**
 * Shared, module-cached sync health for the nav-rail dot and the Settings
 * page. `status.fullySynced` is the green light: configured, nothing
 * pending, nothing failed, and nothing still stamped for a previous central
 * server (which is the "we just changed the kashi URL" case).
 *
 * `sync_state_changed` fires from the Go side whenever the push side
 * advances (a row accepted / failed / re-armed), so the dot updates live
 * without polling.
 */
const status = ref<main.SyncStatusResult | null>(null);
const loading = ref(false);
let loaded = false;

async function refresh() {
  loading.value = true;
  try {
    status.value = await GetSyncStatus();
  } catch (error) {
    console.error("useSyncStatus: GetSyncStatus failed", error);
  } finally {
    loading.value = false;
  }
}

EventsOn("sync_state_changed", () => {
  void refresh();
});
EventsOn("settings_changed", () => {
  void refresh();
});

export function useSyncStatus() {
  if (!loaded) {
    loaded = true;
    void refresh();
  }
  return { status, loading, refresh };
}
