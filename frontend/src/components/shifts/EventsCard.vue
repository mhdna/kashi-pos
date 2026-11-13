<template>
  <!-- One bar across the page, as wide as the cards under it: the count and the
       next event. Tapping it opens the calendar. -->
  <k-sheet class="events-card">
    <div class="events-card-title">
      <button type="button" class="events-card-toggle" data-test="events-open-calendar" @click="router.push('/events')">
        <k-icon icon="mdi-calendar-month-outline" color="grey-darken-1" class="events-card-toggle-icon" />
        <span class="events-card-count text-body-1 font-weight-bold">{{ countLabel }}</span>
        <span v-if="next" class="events-card-next text-body-2 text-medium-emphasis" dir="auto">
          Next: {{ next.name }} · {{ whenLabel(next) }}
        </span>
        <k-spacer />
        <k-icon icon="mdi-chevron-right" class="events-card-chevron" />
      </button>
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

// The folded button counts what is on or starts within this many days.
const COUNT_DAYS = 30;
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

const soonCount = computed(
  () => eventsInRange(today, new Date(today.getTime() + COUNT_DAYS * MS_DAY), synced.value, settings.value).length,
);
// The first of the events the count counts, for the folded bar.
const next = computed(
  () => eventsInRange(today, new Date(today.getTime() + COUNT_DAYS * MS_DAY), synced.value, settings.value)[0],
);
const countLabel = computed(() => {
  const n = soonCount.value;
  if (!n) return `No events in the next ${COUNT_DAYS} days`;
  return `${n} event${n === 1 ? "" : "s"} in the next ${COUNT_DAYS} days`;
});

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
/* The same card as Monthly Shift Income under it, full width. */
.events-card { display: flex; flex-direction: column; width: 100%; min-width: 0; overflow: hidden; border: 1px solid rgba(var(--k-theme-on-surface), 0.12); }
.events-card-title { display: flex; align-items: center; min-width: 0; }
.events-card-toggle {
  flex: 1 1 auto; min-width: 0; display: flex; align-items: center; gap: 10px;
  min-height: 52px; padding: 8px 16px; border: 0; background: transparent; color: inherit;
  font: inherit; text-align: left; cursor: pointer;
}
.events-card-toggle:hover { background: rgba(var(--k-theme-on-surface), 0.04); }
.events-card-toggle:focus-visible { outline: 2px solid rgb(var(--k-theme-primary)); outline-offset: -2px; }
.events-card-toggle-icon, .events-card-chevron { flex: 0 0 auto; }
.events-card-count { flex: 0 0 auto; white-space: nowrap; }
.events-card-next { flex: 0 1 auto; min-width: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
</style>
