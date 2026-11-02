<template>
  <div class="events-page">
    <div class="events-head">
      <k-icon icon="mdi-calendar-month" size="24" color="primary" />
      <h1>Events</h1>
    </div>

    <div class="events-body">
      <section class="upcoming">
        <h2>Coming up</h2>
        <div v-if="!upcoming.length" class="text-medium-emphasis">Nothing in the next {{ UPCOMING_DAYS }} days.</div>
        <div v-for="e in upcoming" :key="e.key" class="upcoming-row">
          <k-avatar size="40" :color="e.color" variant="tonal"><k-icon :icon="e.icon" /></k-avatar>
          <div class="upcoming-text">
            <div class="font-weight-medium">{{ e.name }}</div>
            <div class="text-caption text-medium-emphasis">{{ rangeLabel(e) }}</div>
          </div>
          <k-chip size="small" variant="tonal" :color="e.color">{{ whenLabel(e) }}</k-chip>
        </div>
      </section>

      <section class="month">
        <div class="month-nav">
          <k-btn icon="mdi-chevron-left" variant="text" aria-label="Previous month" @click="shiftMonth(-1)" />
          <div class="month-title">{{ monthTitle }}</div>
          <k-btn icon="mdi-chevron-right" variant="text" aria-label="Next month" @click="shiftMonth(1)" />
          <k-spacer />
          <k-btn variant="tonal" prepend-icon="mdi-calendar-today" @click="goToday">Today</k-btn>
        </div>
        <div class="grid">
          <div v-for="w in weekdays" :key="w" class="grid-head">{{ w }}</div>
          <div v-for="cell in cells" :key="cell.iso" class="cell" :class="{ outside: !cell.inMonth, today: cell.isToday }">
            <div class="cell-num">{{ cell.day }}</div>
            <div v-for="e in cell.events.slice(0, 3)" :key="e.key" class="cell-event" :style="chipStyle(e)" :title="e.name">{{ e.name }}</div>
            <div v-if="cell.events.length > 3" class="cell-more">+{{ cell.events.length - 3 }} more</div>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { ListCalendarEvents } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { eventsInRange, type CalendarEvent, type SyncedEvent } from "@/lib/retailCalendar";
import { onColor, resolveColor } from "@/ui/colors";

const UPCOMING_DAYS = 120;
const MS_DAY = 86_400_000;

const synced = ref<SyncedEvent[]>([]);
const today = ref(startOfDay(new Date()));
const month = ref(new Date(today.value.getFullYear(), today.value.getMonth(), 1));

function startOfDay(d: Date) {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate());
}
const iso = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;

async function load() {
  synced.value = (await ListCalendarEvents()) as SyncedEvent[];
}

const upcoming = computed(() =>
  eventsInRange(today.value, new Date(today.value.getTime() + UPCOMING_DAYS * MS_DAY), synced.value),
);

const fmt = (d: Date) => d.toLocaleDateString(undefined, { day: "numeric", month: "short" });
function rangeLabel(e: CalendarEvent) {
  const sameDay = startOfDay(e.start).getTime() === startOfDay(e.end).getTime();
  return sameDay ? fmt(e.start) : `${fmt(e.start)} – ${fmt(e.end)}`;
}
function whenLabel(e: CalendarEvent) {
  if (e.start <= today.value) return "Now";
  const days = Math.round((startOfDay(e.start).getTime() - today.value.getTime()) / MS_DAY);
  return days === 1 ? "Tomorrow" : `In ${days} days`;
}

const weekdays = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
const monthTitle = computed(() => month.value.toLocaleDateString(undefined, { month: "long", year: "numeric" }));

const cells = computed(() => {
  const first = month.value;
  const lead = (first.getDay() + 6) % 7; // Monday first
  const gridStart = new Date(first.getFullYear(), first.getMonth(), 1 - lead);
  const gridEnd = new Date(gridStart.getFullYear(), gridStart.getMonth(), gridStart.getDate() + 41, 23, 59, 59);
  const events = eventsInRange(gridStart, gridEnd, synced.value);
  return Array.from({ length: 42 }, (_, i) => {
    const date = new Date(gridStart.getFullYear(), gridStart.getMonth(), gridStart.getDate() + i);
    const dayEnd = new Date(date.getFullYear(), date.getMonth(), date.getDate(), 23, 59, 59);
    return {
      iso: iso(date),
      day: date.getDate(),
      inMonth: date.getMonth() === first.getMonth(),
      isToday: date.getTime() === today.value.getTime(),
      events: events.filter((e) => e.end >= date && e.start <= dayEnd),
    };
  });
});

function chipStyle(e: CalendarEvent) {
  return { background: resolveColor(e.color), color: onColor(e.color) };
}
function shiftMonth(by: number) {
  month.value = new Date(month.value.getFullYear(), month.value.getMonth() + by, 1);
}
function goToday() {
  today.value = startOfDay(new Date());
  month.value = new Date(today.value.getFullYear(), today.value.getMonth(), 1);
}

let unsub: (() => void) | undefined;
onMounted(() => {
  void load();
  unsub = EventsOn("sync_applied:calendar_events", () => void load());
});
onBeforeUnmount(() => unsub?.());
</script>

<style scoped>
.events-page { display: flex; flex-direction: column; height: 100%; min-height: 0; padding: 8px 12px; }
.events-head { display: flex; align-items: center; gap: 8px; padding-bottom: 4px; }
.events-head h1 { font-size: 1.15rem; font-weight: 600; line-height: 1.2; margin: 0; }
.events-body { display: flex; gap: 16px; flex: 1 1 auto; min-height: 0; }
.upcoming { flex: 0 0 290px; overflow-y: auto; display: flex; flex-direction: column; gap: 8px; }
.upcoming h2 { font-size: 1rem; font-weight: 600; }
.upcoming-row { display: flex; align-items: center; gap: 12px; padding: 8px 4px; }
.upcoming-text { flex: 1 1 auto; min-width: 0; }
.month { flex: 1 1 auto; min-width: 0; display: flex; flex-direction: column; min-height: 0; }
.month-nav { display: flex; align-items: center; gap: 4px; padding-bottom: 8px; }
.month-title { font-size: 1.1rem; font-weight: 600; min-width: 170px; text-align: center; }
.grid { flex: 1 1 auto; min-height: 0; display: grid; grid-template-columns: repeat(7, 1fr); grid-template-rows: auto repeat(6, 1fr); gap: 1px; background: rgba(128, 128, 128, 0.25); border: 1px solid rgba(128, 128, 128, 0.25); }
.grid-head { background: rgb(var(--k-theme-surface)); padding: 4px; text-align: center; font-size: 0.9rem; font-weight: 600; }
.cell { background: rgb(var(--k-theme-surface)); padding: 3px; min-height: 0; overflow: hidden; display: flex; flex-direction: column; gap: 2px; }
.cell.outside { opacity: 0.45; }
.cell.today .cell-num { background: rgb(var(--k-theme-primary)); color: rgb(var(--k-theme-on-primary)); }
.cell-num { align-self: flex-start; font-size: 0.9rem; min-width: 22px; height: 22px; line-height: 22px; text-align: center; border-radius: 11px; }
.cell-event { font-size: 0.85rem; padding: 1px 6px; border-radius: 4px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.cell-more { font-size: 0.8rem; opacity: 0.7; }
</style>
