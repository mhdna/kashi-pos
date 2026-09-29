<template>
  <v-sheet class="monthly-income-card shifts-card pa-4" style="flex: 2; min-width: 0">
    <div class="monthly-income-title d-flex align-center">
      <v-icon icon="mdi-calendar-month-outline" color="grey-darken-1" class="me-2" />
      <v-card-title class="text-body-1 font-weight-bold px-0">
        Monthly Shift Income ({{ month }})
      </v-card-title>
      <v-btn
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
      <v-btn
        v-if="canChangeMonth"
        icon="mdi-chevron-right"
        variant="text"
        density="compact"
        size="small"
        :disabled="loading || atCurrentMonth"
        aria-label="Next month"
        @click="$emit('next-month')"
      />
      <v-spacer />
      <v-btn
        variant="tonal"
        size="small"
        :prepend-icon="allDaysExpanded ? 'mdi-chevron-up' : 'mdi-chevron-down'"
        :disabled="!rows.length || loading"
        @click="$emit('toggle-all')"
      >
        {{ allDaysExpanded ? "Collapse all" : "Expand all" }}
      </v-btn>
    </div>

    <div class="monthly-income-header text-caption font-weight-bold">
      <span>Day</span>
      <span>Total</span>
    </div>
    <v-progress-linear v-if="loading" indeterminate />
    <div ref="monthlyIncomeScroll" v-else class="monthly-income-scroll">
      <div
        v-for="item in rows"
        :key="item.day"
        :ref="item.day === currentDay ? setCurrentDayRow : undefined"
        class="monthly-income-day"
      >
        <div
          class="monthly-income-row"
          :class="{ 'monthly-income-current-day': item.day === currentDay }"
          @click="$emit('toggle-day', item.day)"
        >
          <v-btn
            class="monthly-income-expand"
            variant="text"
            density="compact"
            size="small"
            :icon="expandedDays.includes(item.day) ? 'mdi-chevron-up' : 'mdi-chevron-down'"
            :aria-label="`${expandedDays.includes(item.day) ? 'Collapse' : 'Expand'} ${item.day}`"
            @click.stop="$emit('toggle-day', item.day)"
          />
          <span class="d-flex align-center ga-2">
            {{ item.day }}
            <v-chip
              v-if="visitorCounterEnabled && visitorCounts[item.day] != null"
              color="orange"
              size="x-small"
              variant="flat"
              :title="`${visitorCounts[item.day]} people visited`"
            >
              <v-icon start icon="mdi-account-group-outline" size="12" />
              {{ visitorCounts[item.day] }}
            </v-chip>
          </span>
          <span class="font-weight-bold">{{ formatBalance(item.total, "USD", currencies, roundingMode) }}</span>
        </div>
        <div v-if="expandedDays.includes(item.day)" class="pa-0">
          <div v-if="!item.shifts.length" class="pa-3 text-caption text-medium-emphasis">
            No data found for this day.
          </div>
          <template v-else>
            <div v-for="shift in item.shifts" :key="shift.id">
              <div
                class="d-flex align-center pa-2 shift-row"
                @click.stop="$emit('toggle-shift', shift.id)"
              >
                <v-chip
                  :color="shift.isClosed ? 'grey' : 'green-darken-2'"
                  size="x-small"
                  variant="outlined"
                  label
                  class="me-2"
                >
                  {{ shift.isClosed ? "Closed" : "Open" }}
                </v-chip>
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
                <v-spacer />
                <span class="font-weight-bold me-2">{{
                  formatBalance(shift.total, "USD", currencies, roundingMode)
                }}</span>
                <v-btn
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
                <v-icon
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
                  <v-table density="compact" class="mb-3">
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
                  </v-table>
                </template>
                <div class="text-caption font-weight-bold mb-1">Balances</div>
                <v-progress-linear v-if="balanceLoading[shift.id]" indeterminate />
                <v-list v-else-if="balanceCache[shift.id]?.length" density="compact" class="pa-0">
                  <v-list-item v-for="balance in balanceCache[shift.id]" :key="balance.accountId">
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
                  </v-list-item>
                </v-list>
                <div v-else class="text-caption text-medium-emphasis">No balance data found.</div>
              </div>
            </div>
          </template>
        </div>
      </div>
    </div>
    <div class="monthly-income-total font-weight-bold">
      <span>Total</span>
      <span>{{ formatBalance(grandTotal, "USD", currencies, roundingMode) }}</span>
    </div>
    <div v-if="deferredCouponTotal > 0" class="text-caption text-medium-emphasis text-end">
      Excludes {{ formatBalance(deferredCouponTotal, "USD", currencies, roundingMode) }} in coupons
      not yet collected
    </div>
    <TargetsProgressCard :targets="targets" class="mt-3" />
  </v-sheet>
</template>

<script setup lang="ts">
import { nextTick, ref, watch, type ComponentPublicInstance } from "vue";
import type { Balance, DayRow, Shift } from "@/types/shifts";
import { formatBalance, formatUsd } from "@/lib/format";
import { formatServerDateTime } from "@/lib/datetime";
import { useCurrencies } from "@/composables/useCurrencies";
import InitialsAvatar from "@/components/InitialsAvatar.vue";
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
  deferredCouponTotal: number;
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
  border: 1px solid rgba(var(--v-theme-on-surface), 0.12);
}

.monthly-income-title {
  min-height: 40px;
  padding-bottom: 8px;
}

.monthly-income-scroll {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
}

.monthly-income-header,
.monthly-income-row,
.monthly-income-total {
  display: grid;
  align-items: center;
  gap: 8px;
}

.monthly-income-header,
.monthly-income-total {
  grid-template-columns: 1fr auto;
}

.monthly-income-row {
  grid-template-columns: 32px 1fr auto;
}

.monthly-income-header {
  min-height: 40px;
  padding: 0 12px;
  border-bottom: 1px solid rgba(128, 128, 128, 0.35);
  color: rgba(var(--v-theme-on-surface), 0.7);
}

.monthly-income-header > span,
.monthly-income-total > span {
  display: inline-flex;
  align-items: center;
}

.monthly-income-row {
  min-height: 40px;
  padding: 0 12px 0 0;
  border-bottom: 1px solid rgba(128, 128, 128, 0.2);
}

.monthly-income-current-day {
  background: rgba(var(--v-theme-on-surface), 0.08);
  border-radius: 8px;
  color: rgb(var(--v-theme-on-surface));
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
  min-height: 40px;
  padding: 0 12px;
  border-top: 1px solid rgba(128, 128, 128, 0.35);
  color: rgb(var(--v-theme-on-surface));
}

.monthly-income-total + .text-caption {
  padding: 0 12px;
  margin-top: -4px;
}
</style>
