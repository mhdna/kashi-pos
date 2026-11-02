<template>
  <k-sheet class="events-card shifts-card pa-4" style="flex: 1; min-width: 0">
    <div class="d-flex align-center events-card-title">
      <k-icon icon="mdi-calendar-month" color="primary" class="me-2" />
      <k-card-title class="text-body-1 font-weight-bold px-0">Coming up</k-card-title>
      <k-spacer />
      <k-btn variant="tonal" size="small" append-icon="mdi-chevron-right" @click="router.push('/events')">Calendar</k-btn>
    </div>
    <div class="events-card-list">
      <div v-if="!upcoming.length" class="text-medium-emphasis text-body-2">Nothing in the next {{ DAYS }} days.</div>
      <div v-for="e in upcoming" :key="e.key" class="events-card-row">
        <k-icon :icon="e.icon" :color="e.color" size="28" class="events-card-icon" />
        <div class="events-card-text">
          <div class="font-weight-medium">{{ e.name }}</div>
          <div class="text-caption text-medium-emphasis">{{ rangeLabel(e) }}</div>
          <k-chip size="small" variant="tonal" :color="e.color" class="mt-1">{{ whenLabel(e) }}</k-chip>
        </div>
      </div>
    </div>
  </k-sheet>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ListCalendarEvents } from "../../../wailsjs/go/main/App";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import { eventsInRange, type CalendarEvent, type SyncedEvent } from "@/lib/retailCalendar";

const DAYS = 90;
const MS_DAY = 86_400_000;
const router = useRouter();
const synced = ref<SyncedEvent[]>([]);
const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate());
const today = startOfDay(new Date());

async function load() {
  try {
    synced.value = (await ListCalendarEvents()) as SyncedEvent[];
  } catch {
    synced.value = [];
  }
}

const upcoming = computed(() =>
  eventsInRange(today, new Date(today.getTime() + DAYS * MS_DAY), synced.value).slice(0, 8),
);

const fmt = (d: Date) => d.toLocaleDateString(undefined, { day: "numeric", month: "short" });
function rangeLabel(e: CalendarEvent) {
  return startOfDay(e.start).getTime() === startOfDay(e.end).getTime() ? fmt(e.start) : `${fmt(e.start)} – ${fmt(e.end)}`;
}
function whenLabel(e: CalendarEvent) {
  if (e.start <= today) return "Now";
  const days = Math.round((startOfDay(e.start).getTime() - today.getTime()) / MS_DAY);
  return days === 1 ? "Tomorrow" : `In ${days} days`;
}

let unsub: (() => void) | undefined;
onMounted(() => {
  void load();
  unsub = EventsOn("sync_applied:calendar_events", () => void load());
});
onBeforeUnmount(() => unsub?.());
</script>

<style scoped>
.shifts-card { height: 100%; min-height: 0; }
.events-card { display: flex; flex-direction: column; overflow: hidden; border: 1px solid rgba(var(--k-theme-on-surface), 0.12); }
.events-card-title { min-height: 40px; padding-bottom: 8px; }
.events-card-list { flex: 1 1 auto; min-height: 0; overflow-y: auto; display: flex; flex-direction: column; gap: 4px; }
.events-card-row { display: flex; align-items: flex-start; gap: 12px; padding: 8px 4px; }
.events-card-icon { flex: 0 0 auto; }
.events-card-text { flex: 1 1 auto; min-width: 0; }
</style>
