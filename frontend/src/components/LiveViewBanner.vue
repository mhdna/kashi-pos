<template>
  <div v-if="controlled" class="live-banner live-banner--control" data-test="live-controlled" role="status">
    Kashi is controlling this screen
  </div>
  <div v-else-if="state.active" class="live-banner live-banner--viewed" data-test="live-viewed" role="status">
    Being viewed
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { liveViewState as state } from "@/lib/liveView";

// A small mark while someone watches, and a clear bar across the top for a few
// seconds after each action a watcher takes on this till.
const tick = ref(Date.now());
let timer: ReturnType<typeof setInterval> | undefined;
watch(
  () => state.controlledUntil,
  (until) => {
    clearInterval(timer);
    tick.value = Date.now();
    if (until > Date.now()) timer = setInterval(() => (tick.value = Date.now()), 500);
  },
  { immediate: true },
);
onBeforeUnmount(() => clearInterval(timer));

const controlled = computed(() => state.active && state.controlledUntil > tick.value);
</script>

<style scoped>
.live-banner {
  position: fixed;
  z-index: 100000;
  font-size: 13px;
  font-weight: 500;
  pointer-events: none;
}
.live-banner--control {
  top: 0;
  left: 0;
  right: 0;
  padding: 6px 12px;
  text-align: center;
  color: #fff;
  background: #c62828;
}
.live-banner--viewed {
  left: 8px;
  bottom: 8px;
  padding: 2px 8px;
  color: #fff;
  background: rgba(0, 0, 0, 0.55);
}
</style>
