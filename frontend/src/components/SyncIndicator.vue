<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { t } from "@/i18n";
import { useAuthStore } from "@/stores/auth";
import { useResyncStore } from "@/stores/resync";
import { useSyncStatus } from "@/composables/useSyncStatus";
import { useLinkStatus } from "@/composables/useLinkStatus";
import { ACTIVITY_HOLD_MS, describeLink, shouldHoldBusy, type Summary } from "@/lib/linkStatus";

// The nav rail's connection light: what the till is doing (spinner, "Products
// 3/21"), whether the internet and the live link to central are up (the live
// link is what central's Branches page shows as connected), and a dialog with
// the detail. Tap it any time to see why.
const auth = useAuthStore();
const router = useRouter();
const resync = useResyncStore();
const { status: syncStatus } = useSyncStatus();
const { link, now } = useLinkStatus();

const summary = computed(() =>
  describeLink(link.value, syncStatus.value, { running: resync.running, progress: resync.progress }, now.value),
);

// What the light shows: the live summary, except that "Synced" waits a moment
// after the last bit of work (see shouldHoldBusy), so one checkout reads as
// one Sending, then Synced, not a flicker.
const shown = ref<Summary>(summary.value);
let holdUntil = 0;
let timer: ReturnType<typeof setTimeout> | null = null;
watch(
  summary,
  (next) => {
    if (timer) clearTimeout(timer);
    timer = null;
    // Only real work (spinning) restarts the hold; Waiting must not hold itself back.
    if (next.spinner.on) holdUntil = Date.now() + ACTIVITY_HOLD_MS;
    if (shouldHoldBusy(next, Date.now(), holdUntil)) {
      timer = setTimeout(() => (shown.value = summary.value), Math.max(0, holdUntil - Date.now()) + 20);
      return;
    }
    shown.value = next;
  },
  { immediate: true },
);
onBeforeUnmount(() => timer && clearTimeout(timer));
const open = ref(false);
const motion = computed(() =>
  shown.value.icon === "mdi-cloud-download" ? "down" : shown.value.icon === "mdi-cloud-upload" ? "up" : "spin",
);

function openSyncSettings() {
  open.value = false;
  router.push({ path: "/settings", query: { panel: "general-sync" } });
}
</script>

<template>
  <div
    :class="['sync-status', `sync-status--${shown.tone}`]"
    :title="shown.title"
    role="button"
    :aria-label="t('Sync: {word}', { word: shown.word })"
    data-test="sync-indicator"
    @click="open = true"
  >
    <!-- The state's icon in its colour: no badge behind it and no motion. While
         work moves, the icon (down for Updating, up for Sending) and the word
         under it say so. -->
    <span
      :class="['sync-status-icon', { 'sync-status-icon--busy': shown.spinner.on }, `sync-status-icon--${motion}`]"
      :data-test="shown.spinner.on ? 'sync-spinner' : undefined"
    >
      <k-icon :icon="shown.icon" size="20" class="sync-status-glyph" />
    </span>
    <span class="sync-status-word" data-test="sync-word">{{ shown.word }}</span>
    <!-- Always takes its line, so the word and the pill below never move. -->
    <span class="sync-status-subslot">
      <span v-if="shown.sub" class="sync-status-sub" data-test="sync-sub">{{ shown.sub }}</span>
    </span>
  </div>

  <k-dialog v-model="open" max-width="520">
    <k-card data-test="sync-dialog">
      <!-- The headline is the state in words; a spinner beside it only while
           work is moving. Each line below is a plain label and its value;
           only a value that needs attention takes a colour. -->
      <k-card-title class="sync-dialog-head pt-4">
        <span>{{ summary.word }}</span>
        <k-progress-circular
          v-if="summary.spinner.on"
          :indeterminate="summary.spinner.pct === null"
          :model-value="summary.spinner.pct ?? 0"
          size="20"
          width="2"
        />
      </k-card-title>
      <k-card-text>
        <p class="sync-dialog-title" data-test="sync-dialog-title">{{ summary.title }}</p>
        <table class="sync-table">
          <tbody>
            <tr v-for="row in summary.rows" :key="row.key" :class="`sync-row sync-row--${row.state}`" :data-test="`sync-row-${row.key}`">
              <th class="sync-row-label" scope="row">{{ row.label }}</th>
              <td>
                <div class="sync-row-value">{{ row.value }}</div>
                <div v-if="row.detail" class="sync-row-detail">{{ row.detail }}</div>
              </td>
            </tr>
          </tbody>
        </table>
      </k-card-text>
      <k-card-actions>
        <k-btn v-if="auth.isAdmin" variant="text" data-test="sync-open-settings" @click="openSyncSettings">{{ t("Sync settings") }}</k-btn>
        <k-spacer />
        <k-btn variant="text" @click="open = false">{{ t("Close") }}</k-btn>
      </k-card-actions>
    </k-card>
  </k-dialog>
</template>

<style scoped>
.sync-status {
  flex: 0 0 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
  padding: 4px 4px 4px;
  cursor: pointer;
  text-align: center;
}
/* One box in every state, so the word under it stays put. */
.sync-status-icon {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border-radius: 50%;
  overflow: visible;
}
/* No animation while it works: the icon (down for Updating, up for Sending)
   and the word say what is happening. */
.sync-status-word {
  font-size: 0.78rem;
  font-weight: 700;
  line-height: 1.15;
  margin-top: 2px;
}
.sync-status-subslot {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 0.8rem;
}
.sync-status-sub {
  font-size: 0.62rem;
  line-height: 1.15;
  opacity: 0.8;
  max-width: 64px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sync-status--ok { color: #66bb6a; }
.sync-status--busy { color: #ffb74d; }
.sync-status--attention { color: #ffd54f; }
.sync-status--bad { color: #ef5350; }
.sync-status--off { color: #9e9e9e; }
.sync-dialog-head { display: flex; align-items: center; gap: 12px; }
.sync-dialog-title { margin: 0 0 16px; opacity: 0.75; }
.sync-table { width: 100%; border-collapse: collapse; }
.sync-table th,
.sync-table td { padding: 8px 0; vertical-align: top; text-align: start; border-top: 1px solid rgba(var(--k-border-color), var(--k-border-opacity)); }
.sync-table tr:first-child th,
.sync-table tr:first-child td { border-top: 0; }
.sync-row-label::first-letter { text-transform: uppercase; }
.sync-row-label { font-weight: 400; opacity: 0.7; padding-inline-end: 24px !important; white-space: nowrap; width: 1%; }
.sync-row-value { font-weight: 500; }
.sync-row-detail { font-size: 0.8rem; opacity: 0.65; margin-top: 2px; }
.sync-row--warn .sync-row-value { color: #b7791f; }
.sync-row--bad .sync-row-value { color: #d32f2f; }
</style>
