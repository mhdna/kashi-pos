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

    <div
      class="monthly-income-header text-caption font-weight-bold"
      :class="{ 'monthly-income--visitors': showVisitors }"
      data-test="income-header"
    >
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
          :class="{ 'monthly-income-current-day': item.day === currentDay, 'monthly-income--visitors': showVisitors }"
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
            {{ item.day }}
            <span v-if="item.day === currentDay" class="mi-today" data-test="today-tag">Today</span>
          </span>
          <!-- How many people came in that day, in a column of its own so the
               numbers line up. -->
          <span v-if="showVisitors" class="mi-visitors-cell">
            <k-chip
              v-if="visitorCounts[item.day] != null"
              class="mi-visitors-chip"
              color="orange"
              size="x-small"
              variant="flat"
              data-test="visitors"
              :title="visitorTitle(item.day, item.total)"
            >
              <k-icon start icon="mdi-account-group-outline" size="12" />
              {{ visitorCounts[item.day] }}
            </k-chip>
          </span>
          <span class="font-weight-bold mi-total">{{ formatBalance(item.total, "USD", currencies, roundingMode) }}</span>
        </div>
        <div v-if="expandedDays.includes(item.day)" class="pa-0">
          <div v-if="!item.shifts.length" class="pa-3 text-caption text-medium-emphasis" data-test="day-empty">
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
    <div class="monthly-income-total font-weight-bold" data-test="income-grand-total">
      <span>Total</span>
      <span>{{ formatBalance(grandTotal, "USD", currencies, roundingMode) }}</span>
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
import { perVisitor } from "@/lib/perVisitor";
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

// What the visitors chip says when hovered: how many people, and what each one
// spent on average.
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

.monthly-income-card {
  /* Compact rows: the width of the first (chevron) column, and the height. */
  --mi-chevron: 32px;
  --mi-row: 30px;
}

.monthly-income-header,
.monthly-income-row,
.monthly-income-total {
  display: grid;
  align-items: center;
  gap: 8px;
}

/* The visitors get a column of their own, the same width in the header and in
   every row, and the total a fixed one (which keeps the visitors a little to
   its left), so the numbers line up. */
.monthly-income-header.monthly-income--visitors {
  grid-template-columns: var(--mi-chevron) 1fr 84px 104px;
}
.monthly-income-row.monthly-income--visitors {
  grid-template-columns: var(--mi-chevron) 1fr 84px 104px;
}
.mi-visitors-head {
  text-align: start;
}
.mi-total {
  text-align: end;
}
/* The header cells are flex boxes, so they are placed with justify-content (a
   text-align would do nothing): Total is centred over its column. */
.mi-total-head {
  justify-content: center;
  text-align: center;
}
.mi-visitors-head {
  justify-content: flex-start;
}
.mi-day {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
/* A small tag on today's row. */
.mi-today {
  padding: 1px 10px;
  border-radius: 999px;
  background: rgb(var(--k-theme-primary));
  color: rgb(var(--k-theme-on-primary));
  font-size: 0.6875rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}
.mi-visitors-chip {
  /* The same width whatever the count (a few people or a thousand), the number
     centred, so the chips make one clean column. */
  width: 68px;
  justify-content: center;
}
.mi-visitors-cell {
  display: flex;
  /* Every pill starts at the same left edge, so they line up down the list. */
  justify-content: flex-start;
}

.monthly-income-header,
.monthly-income-total {
  grid-template-columns: 1fr auto;
}

.monthly-income-row {
  grid-template-columns: var(--mi-chevron) 1fr auto;
}

.monthly-income-header {
  /* The same columns and padding as the rows under it, so Day, Visitors and
     Total sit right over theirs. */
  grid-template-columns: var(--mi-chevron) 1fr auto;
  min-height: 30px;
  padding: 0 12px 0 0;
  border-bottom: 1px solid rgba(128, 128, 128, 0.35);
  color: rgba(var(--k-theme-on-surface), 0.7);
}

.monthly-income-header > span,
.monthly-income-total > span {
  display: inline-flex;
  align-items: center;
}

.monthly-income-row {
  min-height: var(--mi-row);
  padding: 0 12px 0 0;
  border-bottom: 1px solid rgba(128, 128, 128, 0.2);
}

.monthly-income-current-day {
  background: rgba(var(--k-theme-on-surface), 0.08);
  border-radius: 8px;
  color: rgb(var(--k-theme-on-surface));
  font-weight: 600;
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

.monthly-income-total {
  min-height: var(--mi-row);
  padding: 0 12px;
  border-top: 1px solid rgba(128, 128, 128, 0.35);
  color: rgb(var(--k-theme-on-surface));
}

.monthly-income-total + .text-caption {
  padding: 0 12px;
  margin-top: -4px;
}
</style>
