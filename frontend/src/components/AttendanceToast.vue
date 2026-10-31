<template>
  <k-snackbar
    v-model="visible"
    class="attendance-snackbar"
    :timeout="-1"
    location="top"
    variant="flat"
  >
    <div v-if="current" class="attendance-toast" role="status" aria-live="polite">
      <InitialsAvatar :name="current.name" :color="isIn ? 'success' : 'warning'" :size="40" />
      <div class="attendance-toast__text">
        <div class="attendance-toast__name">{{ current.name }}</div>
        <div class="attendance-toast__time">{{ current.time }}</div>
      </div>
      <div class="attendance-toast__kind" :class="isIn ? 'attendance-toast__kind--in' : 'attendance-toast__kind--out'">
        <k-icon icon="mdi-account-clock" size="22" />
        <span>{{ isIn ? "Check in" : "Check out" }}</span>
      </div>
    </div>
  </k-snackbar>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import InitialsAvatar from "@/components/InitialsAvatar.vue";
import { EventsOn } from "../../wailsjs/runtime/runtime";

interface Punch {
  name: string;
  time: string;
  kind: "IN" | "OUT";
}

// How long each snackbar stays up, and the gap before the next queued punch
// appears.
const VISIBLE_MS = 2000;
const GAP_MS = 300;

// `current` keeps its content while the snackbar animates out; `visible`
// drives the snackbar itself.
const current = ref<Punch | null>(null);
const visible = ref(false);
const isIn = computed(() => current.value?.kind === "IN");
const queue: Punch[] = [];
let running = false;
let timer: ReturnType<typeof setTimeout> | undefined;
let off: (() => void) | undefined;

function showNext() {
  const next = queue.shift();
  if (!next) {
    running = false;
    return;
  }
  running = true;
  current.value = next;
  visible.value = true;
  timer = setTimeout(() => {
    visible.value = false;
    timer = setTimeout(showNext, GAP_MS);
  }, VISIBLE_MS);
}

function enqueue(punches: Punch[]) {
  queue.push(...punches);
  if (!running) showNext();
}

onMounted(() => {
  off = EventsOn("attendance_recorded", (payload: { recent?: Punch[] }) => enqueue(payload?.recent ?? []));
});

onUnmounted(() => {
  off?.();
  if (timer) clearTimeout(timer);
});
</script>

<style scoped>
.attendance-toast {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 310px;
}
.attendance-toast__text {
  flex: 1;
  min-width: 0;
}
.attendance-toast__name {
  font-weight: 600;
  line-height: 1.2;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.attendance-toast__time {
  font-size: 0.8rem;
  font-variant-numeric: tabular-nums;
  opacity: 0.75;
}
.attendance-toast__kind {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 1rem;
  font-weight: 700;
  white-space: nowrap;
}
.attendance-toast__kind--in {
  color: rgb(var(--k-theme-success));
}
.attendance-toast__kind--out {
  color: rgb(var(--k-theme-warning));
}
</style>

<style>
/* Same look as the app's other snackbars (home / invoices-list / settings).
   Unscoped: the snackbar is teleported out of this component. Kept local so
   it looks right on every page, not only where that rule happens to be loaded. */
.attendance-snackbar .k-snackbar__wrapper {
  background: rgba(var(--k-theme-surface-variant), 0.72) !important;
  color: rgb(var(--k-theme-on-surface-variant)) !important;
  border: 1px solid rgba(var(--k-theme-on-surface-variant), 0.1);
  box-shadow: 0 4px 18px rgba(0, 0, 0, 0.12) !important;
}
</style>
