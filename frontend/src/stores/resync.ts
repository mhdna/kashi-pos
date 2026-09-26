import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { ResyncAll } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

interface PendingDeactivation {
  productBarcodes: string[];
  productNames: string[];
  clientPhones: string[];
  clientNames: string[];
}

// Module-level (store-owned, not component-owned) so a "Sync everything
// now" run -- and any confirmation dialog it comes back needing -- survives
// navigating away from the Settings page. ResyncAll itself already runs to
// completion on the Go side regardless of the page; this just makes sure
// the UI doesn't lose track of the result (or drop a pending confirmation
// on the floor) when the component that started it gets torn down.
export const useResyncStore = defineStore("resync", () => {
  const running = ref(false);
  const applyStock = ref(false);
  const progress = ref("");
  const error = ref("");
  const report = ref<any>(null);
  const pendingDeactivation = ref<PendingDeactivation | null>(null);
  const showDeactivationConfirm = computed(() => pendingDeactivation.value !== null);
  const deactivationConfirming = ref(false);
  let progressListenerAttached = false;

  function attachProgressListener() {
    if (progressListenerAttached) return;
    progressListenerAttached = true;
    EventsOn("resync_progress", (p: { stage: string; done: number; total: number }) => {
      if (!p || p.stage === "done") {
        progress.value = "";
        return;
      }
      if (p.stage === "pull") {
        progress.value = "Pulling catalog down…";
      } else if (p.total > 0) {
        progress.value = `${p.stage}: ${p.done} / ${p.total}`;
      } else {
        progress.value = `${p.stage}…`;
      }
    });
  }

  async function run(dryRun: boolean, confirmDeactivation = false) {
    running.value = true;
    error.value = "";
    report.value = null;
    try {
      const result = await ResyncAll({
        applyStock: applyStock.value,
        dryRun,
        confirmDeactivation,
      });
      if (result.requiresConfirmation && result.pendingDeactivation) {
        // Nothing was written -- hold the report aside and let the
        // confirmation dialog (rendered wherever the store is read from)
        // ask before re-running with confirmDeactivation: true.
        pendingDeactivation.value = result.pendingDeactivation;
        return;
      }
      report.value = result;
    } catch (e: any) {
      error.value = String(e?.message || e);
    } finally {
      running.value = false;
      progress.value = "";
    }
  }

  async function confirmDeactivationAndRun() {
    deactivationConfirming.value = true;
    try {
      pendingDeactivation.value = null;
      await run(false, true);
    } finally {
      deactivationConfirming.value = false;
    }
  }

  function cancelDeactivation() {
    pendingDeactivation.value = null;
  }

  return {
    running,
    applyStock,
    progress,
    error,
    report,
    pendingDeactivation,
    showDeactivationConfirm,
    deactivationConfirming,
    attachProgressListener,
    run,
    confirmDeactivationAndRun,
    cancelDeactivation,
  };
});
