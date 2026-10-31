<template>
  <div class="conversion-page pa-4">
    <!-- No title: the nav drawer already says Conversion. Nothing to show
         here until a manager has unlocked the page. -->
    <div v-if="unlocked" class="d-flex align-center flex-wrap mb-4 ga-3">
      <k-spacer />
      <template v-if="auth.isAdmin">
        <k-text-field
          v-model="fromDate"
          type="date"
          label="From"
          variant="outlined"
          density="compact"
          hide-details
          style="max-width: 170px"
          @change="refresh"
        />
        <k-text-field
          v-model="toDate"
          type="date"
          label="To"
          variant="outlined"
          density="compact"
          hide-details
          style="max-width: 170px"
          @change="refresh"
        />
      </template>
      <k-chip v-else-if="report" prepend-icon="mdi-calendar-today" size="small" variant="tonal"
        >This month</k-chip
      >
      <k-btn
        v-if="unlocked"
        icon="mdi-refresh"
        variant="text"
        :loading="loading"
        @click="refresh"
      />
    </div>

    <!-- A manager unlocks the page with the manager password each time it's
         opened; it's kept only while the page stays open. -->
    <div v-if="!unlocked" class="unlock">
      <k-card class="pa-6" width="380" rounded="lg" variant="outlined">
        <div class="d-flex align-center ga-2 mb-4">
          <k-icon icon="mdi-lock-outline" />
          <span class="text-subtitle-1 font-weight-bold">Manager password</span>
        </div>
        <k-text-field
          v-model="password"
          autofocus
          density="compact"
          :error-messages="unlockError"
          label="Password"
          type="password"
          variant="outlined"
          @keyup.enter="unlock"
        />
        <k-btn block color="primary" :loading="loading" text="Open" @click="unlock" />
      </k-card>
    </div>

    <template v-else-if="report">
      <k-alert v-if="!visitorCounterOn" class="mb-4" density="compact" type="info" variant="tonal">
        The people counter is off, so new visitors aren't being counted.
      </k-alert>

      <k-alert v-if="report.overall.days === 0" density="compact" type="info" variant="tonal">
        No visitors were counted yet, so there's nothing to compare sales with.
      </k-alert>

      <template v-else>
        <!-- The one number the page is about. -->
        <k-card class="headline mb-4 pa-5" rounded="lg" variant="tonal" color="primary">
          <div class="d-flex align-center ga-5">
            <k-progress-circular :model-value="ratePercent" :size="96" :width="10" color="primary">
              <span class="text-h6 font-weight-bold">{{ ratePercent }}%</span>
            </k-progress-circular>
            <div>
              <div class="text-h5 font-weight-bold text-high-emphasis">
                {{ ratePercent }} of every 100 visitors buy
              </div>
              <div class="text-body-2 text-medium-emphasis mt-1">
                {{ report.overall.invoices }} {{ plural(report.overall.invoices, "sale") }} from
                {{ report.overall.visitors }} {{ plural(report.overall.visitors, "visitor") }}
              </div>
            </div>
          </div>
        </k-card>

        <div class="tiles mb-4">
          <div v-for="tile in tiles" :key="tile.label" class="tile">
            <k-avatar :color="tile.color" rounded="lg" size="40" variant="tonal">
              <k-icon :icon="tile.icon" />
            </k-avatar>
            <div>
              <div class="tile__value">{{ tile.value }}</div>
              <div class="tile__label">{{ tile.label }}</div>
            </div>
          </div>
        </div>

        <!-- What a day with this many visitors should bring, at the rate of
             the days that were about as busy. -->
        <div class="estimate mb-6">
          <k-icon class="text-medium-emphasis" icon="mdi-walk" />
          <span>If</span>
          <k-text-field
            v-model.number="estimateVisitors"
            class="estimate__input"
            density="compact"
            hide-details
            min="0"
            type="number"
            variant="outlined"
          />
          <span>people walk in, expect about</span>
          <strong>{{ estimate.sales }} {{ plural(estimate.sales, "sale") }}</strong>
          <span>and</span>
          <strong>{{ wholeMoney(estimate.revenue) }}</strong>
        </div>
      </template>

      <div class="d-flex align-center mb-2">
        <span class="text-subtitle-1 font-weight-bold">Days</span>
        <k-spacer />
        <span v-if="report.uncountedDays" class="text-caption text-medium-emphasis">
          Days without a visitor count aren't in the figures above
        </span>
      </div>
      <div class="days">
        <!-- Header and rows share one column grid, so every value sits
             under its label whatever the row's last cell holds. -->
        <div class="day day--head">
          <div>Day</div>
          <div class="day__stat"><k-icon icon="mdi-account-group" size="18" />Visitors</div>
          <div class="day__stat"><k-icon icon="mdi-cart-outline" size="18" />Sales</div>
          <div class="day__stat"><k-icon icon="mdi-cash-multiple" size="18" />Total</div>
          <div class="day__rate">Buy rate</div>
        </div>
        <div v-for="day in shownDays" :key="day.day" class="day">
          <div class="day__date">
            <div class="font-weight-medium">{{ dayLabel(day.day) }}</div>
            <div class="text-caption text-medium-emphasis">{{ day.weekday }}</div>
          </div>
          <div class="day__value">{{ day.visitors || "—" }}</div>
          <div class="day__value">{{ day.invoices || "—" }}</div>
          <div class="day__value">{{ day.revenue ? wholeMoney(day.revenue) : "—" }}</div>
          <div class="day__rate">
            <k-chip
              v-if="day.visitors"
              :color="day.invoices ? 'primary' : undefined"
              size="small"
              variant="tonal"
            >
              {{ percent(day.invoices / day.visitors) }}
            </k-chip>
            <span v-else class="text-caption text-medium-emphasis">not counted</span>
          </div>
        </div>
        <div v-if="!shownDays.length" class="text-body-2 text-medium-emphasis pa-2">
          Nothing yet.
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { GetConversionReport } from "../../wailsjs/go/main/App";
import { formatUsd } from "@/lib/format";
import { useAuthStore } from "@/stores/auth";
import { useVisitorCounter } from "@/composables/useVisitorCounter";

type Stats = {
  days: number;
  visitors: number;
  invoices: number;
  revenue: number;
  buyRate: number;
  avgInvoice: number;
  revenuePerVisitor: number;
};
type Group = Stats & { label: string };
type Day = { day: string; weekday: string; visitors: number; invoices: number; revenue: number };
type Report = {
  from: string;
  to: string;
  monthOnly: boolean;
  overall: Stats;
  uncountedDays: number;
  byTraffic: Group[];
  byWeekday: Group[];
  days: Day[];
};

const auth = useAuthStore();
const { enabled: visitorCounterOn } = useVisitorCounter();

function localDay(d: Date) {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}
const now = new Date();
const fromDate = ref(localDay(new Date(now.getFullYear(), now.getMonth(), 1)));
const toDate = ref(localDay(now));

// Admins are let straight in; a manager types the manager password, which
// the server checks on every load (and then shows only this month).
const password = ref("");
const unlocked = ref(auth.isAdmin);
const unlockError = ref("");
const loading = ref(false);
const report = ref<Report | null>(null);

function money(cents: number) {
  return formatUsd((Number(cents) || 0) / 100);
}
function wholeMoney(cents: number) {
  return `$${Math.round((Number(cents) || 0) / 100).toLocaleString()}`;
}
// The counter can miss people, so a day can show more sales than visitors;
// a chance can't be more than certain.
function percent(rate: number) {
  return `${Math.round(Math.min(rate, 1) * 100)}%`;
}
function plural(n: number, word: string) {
  return n === 1 ? word : `${word}s`;
}
function dayLabel(day: string) {
  return new Date(`${day}T00:00:00`).toLocaleDateString(undefined, {
    day: "numeric",
    month: "short",
  });
}

const ratePercent = computed(() =>
  Math.round(Math.min(report.value?.overall.buyRate ?? 0, 1) * 100),
);

const tiles = computed(() => {
  const o = report.value?.overall;
  if (!o) return [];
  return [
    {
      label: "Visitors",
      value: o.visitors.toLocaleString(),
      icon: "mdi-account-group",
      color: "blue",
    },
    {
      label: "Sales",
      value: o.invoices.toLocaleString(),
      icon: "mdi-cart-outline",
      color: "green",
    },
    {
      label: "Average invoice",
      value: money(o.avgInvoice),
      icon: "mdi-receipt-text-outline",
      color: "orange",
    },
    {
      label: "Each visitor brings",
      value: money(o.revenuePerVisitor),
      icon: "mdi-cash-multiple",
      color: "purple",
    },
  ];
});

async function load() {
  loading.value = true;
  try {
    report.value = (await GetConversionReport(
      fromDate.value,
      toDate.value,
      password.value,
    )) as Report;
  } finally {
    loading.value = false;
  }
}

async function unlock() {
  unlockError.value = "";
  try {
    await load();
    unlocked.value = true;
  } catch (error) {
    unlockError.value = String(error).includes("password") ? "Wrong password" : String(error);
  }
}

async function refresh() {
  try {
    await load();
  } catch (error) {
    report.value = null;
    unlockError.value = String(error);
    unlocked.value = auth.isAdmin;
  }
}

// Newest first, and only days something happened.
const shownDays = computed(() =>
  (report.value?.days ?? [])
    .filter((d) => d.visitors || d.invoices)
    .slice()
    .reverse(),
);

// The estimate uses the rate of days about as busy as the number typed in
// (the server's quiet / normal / busy thirds), or the whole range when
// there are too few days to split.
const estimateVisitors = ref(50);
const estimate = computed(() => {
  const r = report.value;
  const n = Math.max(0, Number(estimateVisitors.value) || 0);
  let basis: Stats | undefined = r?.overall;
  const groups = r?.byTraffic ?? [];
  if (groups.length) {
    const days = (r?.days ?? [])
      .filter((d) => d.visitors > 0)
      .map((d) => d.visitors)
      .sort((a, b) => a - b);
    // The busiest day of each third is its upper bound, as the server split them.
    const upper = (third: number) => days[Math.floor((third * days.length) / 3) - 1] ?? 0;
    basis = groups[n <= upper(1) ? 0 : n <= upper(2) ? 1 : 2] ?? basis;
  }
  const rate = Math.min(basis?.buyRate ?? 0, 1);
  return { sales: Math.round(n * rate), revenue: n * rate * (basis?.avgInvoice ?? 0) };
});

onMounted(() => {
  if (unlocked.value) void refresh();
});
</script>

<style scoped>
.conversion-page {
  height: 100%;
  overflow: auto;
}
/* The password card sits in the middle of the page. */
.unlock {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100%;
}
.tiles {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 12px;
}
.tile {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid rgba(var(--k-border-color), var(--k-border-opacity));
  border-radius: 8px;
}
.tile__value {
  font-size: 1.35rem;
  font-weight: 700;
  line-height: 1.2;
}
.tile__label {
  font-size: 0.8rem;
  color: rgba(var(--k-theme-on-surface), var(--k-medium-emphasis-opacity));
}
.estimate {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  padding: 12px 16px;
  border: 1px solid rgba(var(--k-border-color), var(--k-border-opacity));
  border-radius: 8px;
  font-size: 1rem;
}
.estimate__input {
  flex: 0 0 96px;
}
.days {
  border: 1px solid rgba(var(--k-border-color), var(--k-border-opacity));
  border-radius: 8px;
}
.day {
  display: grid;
  grid-template-columns: minmax(110px, 1.4fr) repeat(3, minmax(80px, 1fr)) 110px;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
}
.day--head {
  padding-top: 8px;
  padding-bottom: 8px;
  font-size: 0.8rem;
  color: rgba(var(--k-theme-on-surface), var(--k-medium-emphasis-opacity));
}
.day + .day {
  border-top: 1px solid rgba(var(--k-border-color), var(--k-border-opacity));
}
.day__stat {
  display: flex;
  align-items: center;
  gap: 6px;
}
.day__stat .k-icon {
  opacity: 0.7;
}
/* Values line up under the header's label text, past its icon. */
.day__value {
  padding-left: 24px;
  font-variant-numeric: tabular-nums;
}
.day__rate {
  text-align: right;
}
</style>
