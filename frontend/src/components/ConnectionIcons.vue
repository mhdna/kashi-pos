<script setup lang="ts">
import { computed } from "vue";
import { useResyncStore } from "@/stores/resync";
import { useSyncStatus } from "@/composables/useSyncStatus";
import { useLinkStatus } from "@/composables/useLinkStatus";
import { describeLink } from "@/lib/linkStatus";

// The internet and the live link to central, as two coloured icons. They sit
// in the nav rail's Devices box with the scanner, printer, keyboard and mouse;
// the sync light above says what the till is doing and opens the detail.
const resync = useResyncStore();
const { status: syncStatus } = useSyncStatus();
const { link, now } = useLinkStatus();

const summary = computed(() =>
  describeLink(link.value, syncStatus.value, { running: resync.running, progress: resync.progress }, now.value),
);
</script>

<template>
  <span class="connection-icons">
    <k-icon
      :icon="summary.internetIcon"
      size="20"
      :class="`link-dot link-dot--${summary.internetTone}`"
      :title="summary.internetTone === 'ok' ? 'Internet: connected' : 'Internet: not connected'"
      data-test="sync-internet"
    />
    <k-icon
      :icon="summary.liveIcon"
      size="20"
      :class="`link-dot link-dot--${summary.liveTone}`"
      :title="summary.liveTone === 'ok' ? 'Live link to central: connected' : 'Live link to central: not connected'"
      data-test="sync-live"
    />
  </span>
</template>

<style scoped>
.connection-icons {
  display: contents;
}
.link-dot--ok { color: #66bb6a; }
.link-dot--attention { color: #ffd54f; }
.link-dot--bad { color: #ef5350; }
.link-dot--off { color: #9e9e9e; }
.link-dot--busy { color: #ffb74d; }
</style>
