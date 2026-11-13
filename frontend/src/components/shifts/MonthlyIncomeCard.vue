<template>
  <k-sheet class="monthly-income-card shifts-card pa-3" style="flex: 2; min-width: 0">
    <div class="monthly-income-title d-flex align-center">
      <k-icon icon="mdi-calendar-month-outline" color="grey-darken-1" class="me-2" />
      <k-card-title class="text-body-1 font-weight-bold px-0">
        Monthly Shift Income ({{ month }})
      </k-card-title>
      <k-btn
        v-if="canChangeMonth"
        icon="mdi-chevron-left"
        variant="text"
        density="compact"
        size="small"
        class="ms-1"
        :disabled="loading"
        aria-label="Previous month"
        @click="$emit('prev-month')"
      />
      <k-btn
        v-if="canChangeMonth"
        icon="mdi-chevron-right"
        variant="text"
        density="compact"
        size="small"
        :disabled="loading || atCurrentMonth"
        aria-label="Next month"
        @click="$emit('next-month')"
      />
      <k-spacer />
      <k-btn
        variant="tonal"
        size="small"
        :prepend-icon="allDaysExpanded ? 'mdi-chevron-up' : 'mdi-chevron-down'"
        :disabled="!rows.length || loading"
        @click="$emit('toggle-all')"
      >
        {{ allDaysExpanded ? "Collapse all" : "Expand all" }}
      </k-btn>
    </div>

    <div class="monthly-income-header" :class="{ 'monthly-income--visitors': showVisitors }" data-test="income-header">
      <span />
      <span>Day</span>
      <span v-if="showVisitors" class="mi-visitors-head">Visitors</span>
      <span class="mi-total-head">Total</span>
    </div>
    <k-progress-linear v-if="loading" indeterminate />
    <div ref="monthlyIncomeScroll" v-else class="monthly-income-scroll">
      <div
        v-for="item in rows"
        :key="item.day"
        :ref="item.day === currentDay ? setCurrentDayRow : undefined"
        class="monthly-income-day"
      >
        <div
          class="monthly-income-row"
          :class="[
            { 'monthly-income-current-day': item.day === currentDay, 'monthly-income--visitors': showVisitors },
            `monthly-income-row--${dayKind(item.day, currentDay)}`,
            { 'monthly-income-row--empty': !item.total },
          ]"
          :data-day="item.day"
          @click="$emit('toggle-day', item.day)"
        >
          <k-btn
            class="monthly-income-expand"
            variant="text"
            density="compact"
            size="small"
            :icon="expandedDays.includes(item.day) ? 'mdi-chevron-up' : 'mdi-chevron-down'"
            :aria-label="`${expandedDays.includes(item.day) ? 'Collapse' : 'Expand'} ${item.day}`"
            @click.stop="$emit('toggle-day', item.day)"
          />
          <span class="mi-day">
            <span class="mi-weekday">{{ weekdayShort(item.day) }}</span>
            <span class="mi-date">{{ item.day }}</span>
            <span v-if="item.day === currentDay" class="mi-today">Today</span>
          </span>
          <!-- How many people came in that day, in a column of its own so the
               numbers line up; the tooltip adds what each one spent. -->
          <span v-if="showVisitors" class="mi-visitors-cell">
            <span
              v-if="visitorCounts[item.day] != null"
              class="mi-visitors"
              :class="{ 'mi-visitors--none': !visitorCounts[item.day] }"
              data-test="visitors"
              :title="visitorTitle(item.day, item.total)"
            >
              <k-icon icon="mdi-account-group-outline" size="14" />{{ visitorCounts[item.day] }}
            </span>
          </span>
          <span class="mi-total">{{ totalText(item) }}</span>
        </div>
        <div v-if="expandedDays.includes(item.day)" class="monthly-income-detail">
          <div v-if="!item.shifts.length" class="mi-empty text-caption text-medium-emphasis" data-test="day-empty">
            <k-icon icon="mdi-information-outline" size="16" />
            No data found for this day.
          </div>
          <template v-else>
            <div v-for="shift in item.shifts" :key="shift.id">
              <div
                class="d-flex align-center pa-2 shift-row"
                @click.stop="$emit('toggle-shift', shift.id)"
              >
                <k-chip
                  :color="shift.isClosed ? 'grey' : 'green-darken-2'"
                  size="x-small"
                  variant="outlined"
                  label
                  class="me-2"
                >
                  {{ shift.isClosed ? "Closed" : "Open" }}
                </k-chip>
                <span class="text-caption me-2">#{{ shift.id }}</span>
                <span class="text-caption me-2">{{ formatServerDateTime(shift.createdAt) }}</span>
                <span v-if="shift.closedAt" class="text-caption me-2">→ {{ formatServerDateTime(shift.closedAt) }}</span>
                <InitialsAvatar
                  v-if="shift.closedBySalesperson"
                  :name="shift.closedBySalesperson"
                  :size="24"
                  class="me-2"
                  :title="shift.closedBySalesperson"
                />
                <k-spacer />
                <span class="font-weight-bold me-2">{{
                  formatBalance(shift.total, "USD", currencies, roundingMode)
                }}</span>
                <k-btn
                  v-if="isAdmin && shift.isClosed"
                  icon="mdi-printer-outline"
                  variant="text"
                  density="compact"
                  size="small"
                  title="Print end-of-shift receipt"
                  aria-label="Print end-of-shift receipt"
                  :loading="printingShiftId === shift.id"
                  @click.stop="$emit('print-shift', shift.id)"
                />
                <k-icon
                  :icon="shiftExpanded.has(shift.id) ? 'mdi-chevron-up' : 'mdi-chevron-down'"
                  size="16"
                />
              </div>
              <div v-if="shiftExpanded.has(shift.id)" class="pa-3">
                <div v-if="shift.closedBySalesperson" class="d-flex align-center text-caption text-medium-emphasis mb-3">
                  <span>Closed by: {{ shift.closedBySalesperson }}</span>
                </div>
                <template v-if="shift.isClosed">
                  <div class="text-caption font-weight-bold mb-1">Expected vs Counted</div>
                  <k-table density="compact" class="mb-3">
                    <thead>
                      <tr>
                        <th>Method</th>
                        <th>Expected</th>
                        <th>Counted</th>
                        <th>Variance</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="row in expectedVsCounted(shift)" :key="row.label">
                        <td>{{ row.label }}</td>
                        <td>{{ row.expected }}</td>
                        <td>{{ row.counted }}</td>
                        <td :class="row.varianceCents === 0 ? 'text-medium-emphasis' : row.varianceCents > 0 ? 'text-green-darken-2' : 'text-red-darken-2'">
                          {{ row.variance }}
                        </td>
                      </tr>
                    </tbody>
                  </k-table>
                </template>
                <div class="text-caption font-weight-bold mb-1">Balances</div>
                <k-progress-linear v-if="balanceLoading[shift.id]" indeterminate />
                <k-list v-else-if="balanceCache[shift.id]?.length" density="compact" class="pa-0">
                  <k-list-item v-for="balance in balanceCache[shift.id]" :key="balance.accountId">
                    <template #title>
                      {{ balance.isOnAccount ? "(On Account)" : balance.accountName }}
                      <small v-if="balance.isOnAccount" class="text-medium-emphasis">
                        — not collected</small
                      >
                    </template>
                    <template #append>
                      <span class="font-weight-bold">{{
                        formatBalance(balance.balance, balance.currencyCode ?? "USD", currencies, roundingMode)
                      }}</span>
                    </template>
                  </k-list-item>
                </k-list>
                <div v-else class="text-caption text-medium-emphasis">No balance data found.</div>
              </div>
            </div>
          </template>
        </div>
      </div>
    </div>
    <div class="monthly-income-total" data-test="income-grand-total">
      <span>Total</span>
      <span class="mi-grand">{{ formatBalance(grandTotal, "USD", currencies, roundingMode) }}</span>
    </div>
    <!-- The total counts every sale, as central's reports do; these parts of
         it aren't money in the drawer yet. -->
    <div v-if="couponTotal || onAccountTotal" class="text-caption text-medium-emphasis text-end">
      Includes
      <template v-if="couponTotal">{{ formatBalance(couponTotal, "USD", currencies, roundingMode) }} in coupons</template>
      <template v-if="couponTotal && onAccountTotal"> and </template>
      <template v-if="onAccountTotal">{{ formatBalance(onAccountTotal, "USD", currencies, roundingMode) }} on account</template>,
      not yet collected
    </div>
    <TargetsProgressCard :targets="targets" class="mt-3" />
  </k-sheet>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch, type ComponentPublicInstance } from "vue";
import type { Balance, DayRow, Shift } from "@/types/shifts";
import { formatBalance, formatUsd } from "@/lib/format";
import { formatServerDateTime } from "@/lib/datetime";
import { useCurrencies } from "@/composables/useCurrencies";
import InitialsAvatar from "@/components/InitialsAvatar.vue";
import { dayKind, perVisitor, weekdayShort } from "@/lib/monthlyDays";
import TargetsProgressCard, { type BranchTarget } from "@/components/shifts/TargetsProgressCard.vue";

const { currencies, roundingMode } = useCurrencies();

// LBP's own major-unit figures (already whole LBP, not accounting cents),
// so they format differently from the USD-cents variance rows below.
const lbpSymbol = () => currencies.value.find((c) => c.code === "LBP")?.symbol ?? "LBP";

function usdVarianceRow(label: string, expectedCents: number, countedCents: number) {
  const varianceCents = countedCents - expectedCents;
  return {
    label,
    expected: formatUsd(expectedCents / 100),
    counted: formatUsd(countedCents / 100),
    variance: `${varianceCents >= 0 ? "+" : ""}${formatUsd(varianceCents / 100)}`,
    varianceCents,
  };
}

function expectedVsCounted(shift: Shift) {
  const expectedLbp = shift.expectedLbp ?? 0;
  const countedLbp = shift.countedLbp ?? 0;
  const lbpVariance = countedLbp - expectedLbp;
  return [
    usdVarianceRow("USD Cash", shift.expectedUsd ?? 0, shift.countedUsd ?? 0),
    {
      label: "LBP Cash",
      expected: `${lbpSymbol()} ${expectedLbp.toLocaleString()}`,
      counted: `${lbpSymbol()} ${countedLbp.toLocaleString()}`,
      variance: `${lbpVariance >= 0 ? "+" : ""}${lbpSymbol()} ${lbpVariance.toLocaleString()}`,
      varianceCents: lbpVariance,
    },
    usdVarianceRow("Visa", shift.expectedVisa ?? 0, shift.countedVisa ?? 0),
    usdVarianceRow("Whish", shift.expectedWhish ?? 0, shift.countedWhish ?? 0),
  ];
}

const props = defineProps<{
  month: string;
  rows: DayRow[];
  grandTotal: number;
  // Parts of grandTotal not yet collected: "calculate on checkout" coupons
  // and sales on account (shifts.vue, GetMonthSales).
  couponTotal: number;
  onAccountTotal: number;
  loading: boolean;
  expandedDays: string[];
  allDaysExpanded: boolean;
  shiftExpanded: Set<number>;
  balanceCache: Record<number, Balance[]>;
  balanceLoading: Record<number, boolean>;
  currentDay: string;
  isAdmin: boolean;
  canChangeMonth: boolean;
  printingShiftId: number | null;
  targets: BranchTarget[];
  visitorCounts: Record<string, number>;
  visitorCounterEnabled: boolean;
  atCurrentMonth: boolean;
}>();

const showVisitors = computed(() => props.visitorCounterEnabled);

// A day to come with nothing in it shows a dash, not a $0.00 that reads as a
// day that sold nothing.
function totalText(item: DayRow): string {
  if (!item.total && dayKind(item.day, props.currentDay) === "future") return "—";
  return formatBalance(item.total, "USD", currencies.value, roundingMode.value);
}

function visitorTitle(day: string, total: number): string {
  const count = props.visitorCounts[day];
  if (count == null) return "";
  const people = `${count} ${count === 1 ? "person" : "people"} visited`;
  const each = perVisitor(total, count);
  return each === null ? people : `${people} · ${formatBalance(each, "USD", currencies.value, roundingMode.value)} per visitor`;
}

const monthlyIncomeScroll = ref<HTMLElement | null>(null);
const currentDayRow = ref<HTMLElement | null>(null);

function setCurrentDayRow(element: Element | ComponentPublicInstance | null) {
  currentDayRow.value = element instanceof HTMLElement ? element : null;
}

async function centerCurrentDay() {
  if (props.loading || !props.rows.length) return;

  await nextTick();
  const index = props.rows.findIndex((row) => row.day === props.currentDay);
  if (index < 0) return;

  requestAnimationFrame(() => {
    const row = currentDayRow.value;
    const container = monthlyIncomeScroll.value;
    if (!row || !container) return;

    const centeredOffset = Math.max((container.clientHeight - row.offsetHeight) / 2, 0);
    container.scrollTop +=
      row.getBoundingClientRect().top - container.getBoundingClientRect().top - centeredOffset;
  });
}

watch(() => [props.loading, props.rows.length, props.currentDay], centerCurrentDay, {
  immediate: true,
});

const emit = defineEmits<{
  "toggle-all": [];
  "prev-month": [];
  "next-month": [];
  "toggle-day": [day: string];
  "toggle-shift": [shiftId: number];
  "print-shift": [shiftId: number];
}>();
</script>

<style scoped>
.shifts-card {
  height: 100%;
  min-height: 0;
}

.monthly-income-card {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid rgba(var(--k-theme-on-surface), 0.12);
}

.monthly-income-title {
  min-height: 30px;
  padding-bottom: 4px;
}

.monthly-income-scroll {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
}

.monthly-income-header,
.monthly-income-row {
  display: grid;
  align-items: center;
  gap: 8px;
  grid-template-columns: 32px 1fr auto;
}
/* A column for the visitors, the same width in the header and every row. */
.monthly-income--visitors {
  grid-template-columns: 32px 1fr 76px auto;
}

.monthly-income-total {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.monthly-income-header {
  min-height: 32px;
  padding: 0 12px 0 0;
  border-bottom: 1px solid rgba(var(--k-theme-on-surface), 0.16);
  color: rgba(var(--k-theme-on-surface), 0.6);
  font-size: 0.6875rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}
.mi-visitors-head {
  text-align: center;
}
.mi-total-head {
  text-align: end;
}

.monthly-income-row {
  position: relative;
  min-height: 38px;
  padding: 0 12px 0 0;
  border-bottom: 1px solid rgba(var(--k-theme-on-surface), 0.07);
  cursor: pointer;
  transition: background-color 0.12s ease;
}
.monthly-income-row:hover {
  background: rgba(var(--k-theme-on-surface), 0.05);
}

/* The date, with its weekday before it in a quiet tone. */
.mi-day {
  display: flex;
  align-items: baseline;
  gap: 10px;
  min-width: 0;
}
.mi-weekday {
  width: 28px;
  flex: 0 0 auto;
  color: rgba(var(--k-theme-on-surface), 0.5);
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
}
.mi-date {
  font-variant-numeric: tabular-nums;
}
.mi-today {
  align-self: center;
  padding: 1px 8px;
  border-radius: 999px;
  background: rgb(var(--k-theme-primary));
  color: rgb(var(--k-theme-on-primary));
  font-size: 0.625rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.mi-visitors-cell {
  display: flex;
  justify-content: center;
}
/* A small pill: people icon and the count, in the warning tone but soft. */
.mi-visitors {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 9px;
  border-radius: 999px;
  background: color-mix(in srgb, rgb(var(--k-theme-warning)) 20%, transparent);
  color: color-mix(in srgb, rgb(var(--k-theme-warning)) 70%, rgb(var(--k-theme-on-surface)));
  font-size: 0.8125rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.mi-visitors--none {
  background: transparent;
  color: rgba(var(--k-theme-on-surface), 0.4);
  font-weight: 500;
}

.mi-total {
  text-align: end;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
/* A day with nothing sold keeps its place but steps back, so the days that did
   sell stand out; a day still to come steps back further. */
.monthly-income-row--empty .mi-total {
  color: rgba(var(--k-theme-on-surface), 0.4);
  font-weight: 500;
}
.monthly-income-row--future {
  opacity: 0.6;
}

/* Today: a tinted row with an accent bar down its left edge. */
.monthly-income-current-day {
  background: rgba(var(--k-theme-primary), 0.09);
  box-shadow: inset 3px 0 0 rgb(var(--k-theme-primary));
  font-weight: 600;
}
.monthly-income-current-day:hover {
  background: rgba(var(--k-theme-primary), 0.13);
}
.monthly-income-current-day .mi-weekday {
  color: rgb(var(--k-theme-primary));
}

/* What a day opens to, set in from the left under its own line. */
.monthly-income-detail {
  margin: 0 0 6px 16px;
  border-left: 2px solid rgba(var(--k-theme-on-surface), 0.12);
  background: rgba(var(--k-theme-on-surface), 0.025);
}
.mi-empty {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 12px;
}

.monthly-income-total {
  min-height: 44px;
  margin-top: 6px;
  padding: 0 12px;
  border-radius: 8px;
  background: rgba(var(--k-theme-on-surface), 0.06);
  color: rgb(var(--k-theme-on-surface));
  font-weight: 700;
}
.mi-grand {
  font-size: 1.125rem;
  font-variant-numeric: tabular-nums;
}

.shift-row {
  border-bottom: 1px solid rgba(128, 128, 128, 0.2);
  cursor: pointer;
}

.monthly-income-expand {
  grid-column: 1;
  grid-row: 1;
  justify-self: start;
}

.monthly-income-total + .text-caption {
  padding: 0 12px;
  margin-top: -4px;
}
</style>
