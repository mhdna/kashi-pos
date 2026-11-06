<template>
  <k-sheet class="events-card px-4 py-2" style="min-width: 0">
    <div class="d-flex align-center events-card-title">
      <k-icon icon="mdi-calendar-month" color="primary" class="me-2" />
      <k-card-title class="text-body-1 font-weight-bold px-0">Coming up</k-card-title>
      <k-spacer />
      <k-btn variant="tonal" size="small" append-icon="mdi-chevron-right" @click="router.push('/events')">Calendar</k-btn>
    </div>
    <div class="events-card-list">
      <div v-if="!upcoming.length" class="text-medium-emphasis text-body-2">Nothing in the next {{ DAYS }} days.</div>
      <div v-for="e in upcoming" :key="e.key" class="events-card-row">
        <k-icon :icon="e.icon" :color="e.color" size="24" class="events-card-icon" />
        <div class="events-card-text">
          <div class="font-weight-medium text-body-2 events-card-name" dir="auto" :title="e.name">{{ e.name }}</div>
          <div class="events-card-meta text-caption text-medium-emphasis">
            <span>{{ rangeLabel(e) }}</span>
            <k-chip size="x-small" variant="tonal" :color="e.color">{{ whenLabel(e) }}</k-chip>
          </div>
        </div>
      </div>
    </div>
  </k-sheet>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { GetCalendarSettings, ListCalendarEvents } from "../../../wailsjs/go/main/App";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import {
  DEFAULT_CALENDAR_SETTINGS,
  eventsInRange,
  type CalendarEvent,
  type CalendarSettings,
  type SyncedEvent,
} from "@/lib/retailCalendar";

const DAYS = 90;
const MS_DAY = 86_400_000;
const router = useRouter();
const synced = ref<SyncedEvent[]>([]);
const settings = ref<CalendarSettings>(DEFAULT_CALENDAR_SETTINGS);
const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate());
const today = startOfDay(new Date());

async function load() {
  try {
    synced.value = (await ListCalendarEvents()) as SyncedEvent[];
    settings.value = (await GetCalendarSettings()) as CalendarSettings;
  } catch {
    synced.value = [];
  }
}

const upcoming = computed(() =>
  eventsInRange(today, new Date(today.getTime() + DAYS * MS_DAY), synced.value, settings.value).slice(0, 6),
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
  const offEvents = EventsOn("sync_applied:calendar_events", () => void load());
  const offSettings = EventsOn("sync_applied:calendar_settings", () => void load());
  unsub = () => {
    offEvents();
    offSettings();
  };
});
onBeforeUnmount(() => unsub?.());
</script>

<style scoped>
.events-card { display: flex; flex-direction: column; overflow: hidden; border: 1px solid rgba(var(--k-theme-on-surface), 0.12); }
.events-card-title { min-height: 32px; padding-bottom: 2px; }
.events-card-list { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 0 12px; }
.events-card-row { min-width: 0; display: flex; align-items: center; gap: 8px; padding: 2px 2px; }
.events-card-name { line-height: 1.25; text-align: left; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.events-card-meta { display: flex; flex-wrap: wrap; align-items: center; gap: 2px 6px; }
.events-card-icon { flex: 0 0 auto; }
.events-card-text { flex: 1 1 auto; min-width: 0; }
</style>
