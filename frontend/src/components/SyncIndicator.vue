<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { useRouter } from "vue-router";
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
    if (next.spinner.on || next.tone === "busy") holdUntil = Date.now() + ACTIVITY_HOLD_MS;
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

function openSyncSettings() {
  open.value = false;
  router.push({ path: "/settings", query: { panel: "general-sync" } });
}
const rowIcon = { ok: "mdi-check-circle", warn: "mdi-alert-circle-outline", bad: "mdi-close-circle", idle: "mdi-circle-outline" } as const;
</script>

<template>
  <div
    :class="['sync-status', `sync-status--${shown.tone}`]"
    :title="shown.title"
    role="button"
    :aria-label="`Sync: ${shown.word}`"
    data-test="sync-indicator"
    @click="open = true"
  >
    <span class="sync-status-icon">
      <k-progress-circular
        v-if="shown.spinner.on"
        :indeterminate="shown.spinner.pct === null"
        :model-value="shown.spinner.pct ?? 0"
        size="26"
        width="3"
        color="#ffb74d"
        data-test="sync-spinner"
      >
        <k-icon :icon="shown.icon" size="14" />
      </k-progress-circular>
      <k-icon v-else :icon="shown.icon" size="22" />
    </span>
    <span class="sync-status-word" data-test="sync-word">{{ shown.word }}</span>
    <!-- Always takes its line, so the word and the pill below never move. -->
    <span class="sync-status-subslot">
      <span v-if="shown.sub" class="sync-status-sub" data-test="sync-sub">{{ shown.sub }}</span>
    </span>
    <span class="sync-status-links">
      <k-icon
        :icon="shown.internetIcon"
        size="16"
        :class="`link-dot link-dot--${shown.internetTone}`"
        data-test="sync-internet"
      />
      <k-icon
        :icon="shown.liveIcon"
        size="16"
        :class="`link-dot link-dot--${shown.liveTone}`"
        data-test="sync-live"
      />
    </span>
  </div>

  <k-dialog v-model="open" max-width="520">
    <k-card rounded="xl" data-test="sync-dialog">
      <k-card-title class="d-flex align-center ga-3 pt-4">
        <k-progress-circular
          v-if="summary.spinner.on"
          :indeterminate="summary.spinner.pct === null"
          :model-value="summary.spinner.pct ?? 0"
          size="28"
          width="3"
        />
        <k-icon v-else :icon="summary.icon" size="28" :class="`link-dot link-dot--${summary.tone}`" />
        <span>{{ summary.word }}</span>
      </k-card-title>
      <k-card-text>
        <p class="sync-dialog-title" data-test="sync-dialog-title">{{ summary.title }}</p>
        <ul class="sync-rows">
          <li v-for="row in summary.rows" :key="row.key" :class="`sync-row sync-row--${row.state}`" :data-test="`sync-row-${row.key}`">
            <k-icon :icon="rowIcon[row.state]" size="18" class="sync-row-icon" />
            <div>
              <div class="sync-row-head">
                <span class="sync-row-label">{{ row.label }}</span>
                <span class="sync-row-value">{{ row.value }}</span>
              </div>
              <div v-if="row.detail" class="sync-row-detail">{{ row.detail }}</div>
            </div>
          </li>
        </ul>
      </k-card-text>
      <k-card-actions>
        <k-btn v-if="auth.isAdmin" variant="text" data-test="sync-open-settings" @click="openSyncSettings">Sync settings</k-btn>
        <k-btn variant="text" @click="open = false">Close</k-btn>
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
/* The ring and the plain icon get the same box, so the word under them stays put. */
.sync-status-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
}
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
/* The two connection icons sit together in one quiet pill. */
.sync-status-links {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin-top: 2px;
  padding: 3px 9px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.07);
}
.sync-status--ok { color: #66bb6a; }
.sync-status--busy { color: #ffb74d; }
.sync-status--attention { color: #ffd54f; }
.sync-status--bad { color: #ef5350; }
.sync-status--off { color: #9e9e9e; }
.link-dot--ok { color: #66bb6a; }
.link-dot--attention { color: #ffd54f; }
.link-dot--bad { color: #ef5350; }
.link-dot--off { color: #9e9e9e; }
.link-dot--busy { color: #ffb74d; }
.sync-dialog-title { margin: 0 0 12px; opacity: 0.9; }
.sync-rows { list-style: none; margin: 0; padding: 0; display: grid; gap: 10px; }
.sync-row { display: flex; gap: 10px; align-items: flex-start; }
.sync-row-head { display: flex; gap: 8px; flex-wrap: wrap; }
.sync-row-label { font-weight: 600; }
.sync-row-detail { font-size: 0.8rem; opacity: 0.75; }
.sync-row--ok .sync-row-icon { color: #66bb6a; }
.sync-row--warn .sync-row-icon { color: #ffd54f; }
.sync-row--bad .sync-row-icon { color: #ef5350; }
.sync-row--idle .sync-row-icon { color: #9e9e9e; }
</style>
