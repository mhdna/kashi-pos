<script setup lang="ts">
import { computed, ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useResyncStore } from "@/stores/resync";
import { useSyncStatus } from "@/composables/useSyncStatus";
import { useLinkStatus } from "@/composables/useLinkStatus";
import { describeLink } from "@/lib/linkStatus";

// The nav rail's connection light: what the till is doing (spinner, "Products
// 3/21"), whether the internet and the live link to kashi are up (the live
// link is what kashi's Branches page shows as connected), and a dialog with
// the detail. Tap it any time to see why.
const auth = useAuthStore();
const router = useRouter();
const resync = useResyncStore();
const { status: syncStatus } = useSyncStatus();
const { link, now } = useLinkStatus();

const summary = computed(() =>
  describeLink(link.value, syncStatus.value, { running: resync.running, progress: resync.progress }, now.value),
);
const open = ref(false);

function openSyncSettings() {
  open.value = false;
  router.push({ path: "/settings", query: { panel: "general-sync" } });
}
const rowIcon = { ok: "mdi-check-circle", warn: "mdi-alert-circle-outline", bad: "mdi-close-circle", idle: "mdi-circle-outline" } as const;
</script>

<template>
  <div
    :class="['sync-status', `sync-status--${summary.tone}`]"
    :title="summary.title"
    role="button"
    :aria-label="`Sync: ${summary.word}`"
    data-test="sync-indicator"
    @click="open = true"
  >
    <span class="sync-status-icon">
      <k-progress-circular
        v-if="summary.spinner.on"
        :indeterminate="summary.spinner.pct === null"
        :model-value="summary.spinner.pct ?? 0"
        size="26"
        width="3"
        color="#ffb74d"
        data-test="sync-spinner"
      >
        <k-icon :icon="summary.icon" size="14" />
      </k-progress-circular>
      <k-icon v-else :icon="summary.icon" size="22" />
    </span>
    <span class="sync-status-word" data-test="sync-word">{{ summary.word }}</span>
    <span v-if="summary.sub" class="sync-status-sub" data-test="sync-sub">{{ summary.sub }}</span>
    <span class="sync-status-links">
      <k-icon
        :icon="summary.internetIcon"
        size="14"
        :class="`link-dot link-dot--${summary.internetTone}`"
        data-test="sync-internet"
      />
      <k-icon
        :icon="summary.liveIcon"
        size="14"
        :class="`link-dot link-dot--${summary.liveTone}`"
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
  gap: 2px;
  padding: 8px 2px;
  cursor: pointer;
  text-align: center;
}
.sync-status-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 26px;
}
.sync-status-word {
  font-size: 0.7rem;
  font-weight: 700;
  line-height: 1.1;
}
.sync-status-sub {
  font-size: 0.6rem;
  line-height: 1.1;
  opacity: 0.85;
  max-width: 64px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sync-status-links {
  display: flex;
  gap: 6px;
  margin-top: 2px;
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
