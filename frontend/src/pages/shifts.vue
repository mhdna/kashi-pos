<template>
  <div class="shifts-page d-flex flex-column h-100">
    <div class="shifts-cards d-flex">
      <ShiftIncomeCard
        :items="shiftIncome"
        title="Current Shift Income"
        :shift-id="openShift?.id"
        :income-total="currentShiftTotal"
        :loading="loadingShift"
        :payment-methods="paymentMethods"
        :balance-sources="closeBalances"
        :opening-float-usd="openShift?.openingFloatUsd"
        :opening-float-lbp="openShift?.openingFloatLbp"
        :opening-float-enabled="openingFloatEnabled"
        @balance-replaced="refresh"
        @opening-float-set="refresh"
      />
      <MonthlyIncomeCard
        :month="currentMonth"
        :rows="dailyRows"
        :grand-total="grandTotal"
        :deferred-coupon-total="deferredCouponTotal"
        :loading="loadingMonth"
        :expanded-days="expandedDays"
        :all-days-expanded="allDaysExpanded"
        :shift-expanded="shiftExpanded"
        :balance-cache="balanceCache"
        :balance-loading="balanceLoading"
        :current-day="currentDay"
        :is-admin="auth.isAdmin"
        :can-change-month="canChangeMonth"
        :printing-shift-id="printingShiftId"
        :targets="visibleTargets"
        :visitor-counts="visitorCounts"
        :visitor-counter-enabled="visitorCounterEnabled"
        @toggle-all="toggleAllDays"
        @toggle-day="toggleDay"
        @toggle-shift="toggleShift"
        :at-current-month="currentMonth === thisMonthKey"
        @print-shift="printClosedShift"
        @prev-month="() => stepMonth(-1)"
        @next-month="() => stepMonth(1)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import {
  GetAppSettings,
  GetCurrentShiftIncomeByAccount,
  GetCurrentMonth,
  GetDeferredCouponTotalByMonth,
  GetOpenShift,
  GetShiftTotals,
  ListBranchTargetsWithProgress,
  ListShiftBalances,
  ListPaymentMethods,
  PrintClosedShiftReceipt,
  GetVisitorCountsForMonth,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import MonthlyIncomeCard from "@/components/shifts/MonthlyIncomeCard.vue";
import ShiftIncomeCard from "@/components/shifts/ShiftIncomeCard.vue";
import { INCOME_CHANGED_EVENT } from "@/lib/incomeEvents";
import type { Balance, DayRow, IncomeRow, PaymentMethod, Shift } from "@/types/shifts";
import { useVisitorCounter } from "@/composables/useVisitorCounter";
import { useAuthStore } from "@/stores/auth";
import { serverDateLocalDayKey } from "@/lib/datetime";
const auth = useAuthStore();
const { enabled: visitorCounterEnabled, count: visitorCounterToday } = useVisitorCounter();
const paymentMethods = ref<PaymentMethod[]>([]);
const openingFloatEnabled = ref(true);
const deferredCouponTotal = ref(0);
const nonAdminMonthNavEnabled = ref(false);
const canChangeMonth = computed(() => auth.isAdmin || nonAdminMonthNavEnabled.value);
// Keyed "DD/MM/YYYY" to match the day keys MonthlyIncomeCard renders.
const visitorCounts = ref<Record<string, number>>({});

const openShift = ref<Shift | null>(null);
const allShifts = ref<Shift[]>([]);
const currentMonth = ref("");
const thisMonthKey = `${new Date().getFullYear()}-${pad(new Date().getMonth() + 1)}`;

function stepMonth(delta: number) {
  if (!canChangeMonth.value) return;
  const [y, m] = currentMonth.value.split("-").map(Number);
  if (!y || !m) return;
  const d = new Date(y, m - 1 + delta, 1);
  const next = `${d.getFullYear()}-${pad(d.getMonth() + 1)}`;
  if (next > thisMonthKey) return; // don't page into the future
  currentMonth.value = next;
  void refresh();
}
const loadingShift = ref(true);
const loadingMonth = ref(true);
const printingShiftId = ref<number | null>(null);

const balanceCache = reactive<Record<number, Balance[]>>({});
const balanceLoading = reactive<Record<number, boolean>>({});
const shiftExpanded = reactive(new Set<number>());
const expandedDays = ref<string[]>([]);
const currentDay = computed(() => {
  const now = new Date();
  return `${pad(now.getDate())}/${pad(now.getMonth() + 1)}/${now.getFullYear()}`;
});

// Reflect nav-drawer up/down clicks straight into today's chip without a
// full page reload - the composable's `count` is the shared source of truth.
watch(visitorCounterToday, (today) => {
  visitorCounts.value = { ...visitorCounts.value, [currentDay.value]: today };
});
const shiftIncome = computed<IncomeRow[]>(() =>
  todayIncome.value.map((income) => ({
    accountId: Number(income.accountId) || 0,
    accountName: income.accountName,
    currencyCode: income.currencyCode,
    color: income.color,
    total: Number(income.total) || 0,
    isOnAccount: income.isOnAccount,
    isCollected: income.isCollected,
  })),
);

const dailyRows = computed<DayRow[]>(() => {
  const groups: Record<string, Shift[]> = {};
  for (const shift of allShifts.value) {
    const day = serverDateLocalDayKey(shift.createdAt);
    groups[day] ??= [];
    groups[day].push(shift);
  }

  for (const shifts of Object.values(groups)) {
    shifts.sort((left, right) => right.id - left.id);
  }

  const [year, month] = currentMonth.value.split("-");
  if (!year || !month) {
    return Object.entries(groups).map(([day, shifts]) => ({
      day,
      total: dayTotal(shifts),
      shifts,
    }));
  }

  const daysInMonth = new Date(+year, +month, 0).getDate();
  return Array.from({ length: daysInMonth }, (_, index) => {
    const day = `${pad(index + 1)}/${month}/${year}`;
    const shifts = groups[day] ?? [];
    return { day, total: dayTotal(shifts), shifts };
  });
});

const grandTotal = computed(() => dailyRows.value.reduce((total, row) => total + row.total, 0));
const currentShiftTotal = computed(() => {
  const shiftId = Number(openShift.value?.id) || 0;
  if (!shiftId) return 0;
  return Number(allShifts.value.find((shift) => shift.id === shiftId)?.total) || 0;
});
const allDaysExpanded = computed(
  () =>
    dailyRows.value.length > 0 &&
    dailyRows.value.every((row) => expandedDays.value.includes(row.day)),
);

function dayTotal(shifts: Shift[]) {
  return shifts.reduce((total, shift) => total + shift.total, 0);
}

function pad(value: number) {
  return value.toString().padStart(2, "0");
}


function toggleDay(day: string) {
  expandedDays.value = expandedDays.value.includes(day)
    ? expandedDays.value.filter((value) => value !== day)
    : [...expandedDays.value, day];
}

function toggleAllDays() {
  expandedDays.value = allDaysExpanded.value ? [] : dailyRows.value.map((row) => row.day);
}

async function toggleShift(shiftId: number) {
  if (shiftExpanded.has(shiftId)) {
    shiftExpanded.delete(shiftId);
    return;
  }

  shiftExpanded.add(shiftId);
  if (!balanceCache[shiftId]) await loadBalances(shiftId);
}

async function printClosedShift(shiftId: number) {
  if (!auth.isAdmin || printingShiftId.value !== null) return;
  printingShiftId.value = shiftId;
  try {
    await PrintClosedShiftReceipt(shiftId);
    window.alert("End-of-shift receipt sent to the printer");
  } catch (error) {
    window.alert(error instanceof Error ? error.message : String(error));
  } finally {
    printingShiftId.value = null;
  }
}

async function loadBalances(shiftId: number) {
  balanceLoading[shiftId] = true;
  try {
    balanceCache[shiftId] = ((await ListShiftBalances(shiftId)) as Balance[]) ?? [];
  } catch {
    balanceCache[shiftId] = [];
  } finally {
    balanceLoading[shiftId] = false;
  }
}

const closeBalances = ref<Balance[]>([]);
const todayIncome = ref<IncomeRow[]>([]);
const branchTargets = ref<
  { id: number; dateFrom: string; dateTo: string; targetAmountCents: number; achievedCents: number; color: string }[]
>([]);

// Only surface targets that cover the selected month — either they sit
// entirely within it, or their date range spills across it.
const visibleTargets = computed(() => {
  const [year, month] = currentMonth.value.split("-").map(Number);
  if (!year || !month) return [];
  const monthStart = new Date(year, month - 1, 1);
  const monthEnd = new Date(year, month, 0, 23, 59, 59, 999);
  return branchTargets.value.filter((t) => {
    const from = new Date(t.dateFrom);
    const to = new Date(t.dateTo);
    return from <= monthEnd && to >= monthStart;
  });
});

async function refresh() {
  loadingShift.value = true;
  loadingMonth.value = true;

  try {
    openShift.value = (await GetOpenShift()) as Shift | null;
    if (openShift.value?.id) {
      closeBalances.value = ((await ListShiftBalances(openShift.value.id)) as Balance[]) ?? [];
    } else {
      closeBalances.value = [];
    }
  } catch {
    openShift.value = null;
    closeBalances.value = [];
  }

  try {
    todayIncome.value = ((await GetCurrentShiftIncomeByAccount()) as IncomeRow[]) ?? [];
  } catch {
    todayIncome.value = [];
  }

  try {
    const shifts = ((await GetShiftTotals(currentMonth.value)) as Shift[]) ?? [];
    allShifts.value = shifts.map((shift) => ({
      ...shift,
      total: Number.isFinite(Number(shift.total)) ? Number(shift.total) : 0,
    }));
  } catch {
    allShifts.value = [];
  }

  try {
    const excluded = await GetDeferredCouponTotalByMonth(currentMonth.value);
    deferredCouponTotal.value = Number.isFinite(Number(excluded)) ? Number(excluded) : 0;
  } catch {
    deferredCouponTotal.value = 0;
  }

  for (const shiftId of Object.keys(balanceCache)) delete balanceCache[Number(shiftId)];
  await Promise.all([...shiftExpanded].map(loadBalances));

  try {
    branchTargets.value = ((await ListBranchTargetsWithProgress()) ?? []) as typeof branchTargets.value;
  } catch {
    branchTargets.value = [];
  }

  try {
    const byIsoDay = ((await GetVisitorCountsForMonth(currentMonth.value)) ?? {}) as Record<string, number>;
    const remapped: Record<string, number> = {};
    for (const [isoDay, value] of Object.entries(byIsoDay)) {
      const [year, month, day] = isoDay.split("-");
      if (year && month && day) remapped[`${day}/${month}/${year}`] = value;
    }
    visitorCounts.value = remapped;
  } catch {
    visitorCounts.value = {};
  }

  loadingShift.value = false;
  loadingMonth.value = false;
}

function refreshIncome() {
  void refresh();
}

// Backend sync (poll or WS push) applies changes straight to the local
// DB with no way for an already-open page to know its data went stale --
// this reuses that same signal to trigger a reload, same as a local
// mutation firing INCOME_CHANGED_EVENT does. See sync_applied:<entity> in
// kashi-pos's sync_pull.go.
let stopSyncListeners: (() => void)[] = [];

onMounted(async () => {
  window.addEventListener(INCOME_CHANGED_EVENT, refreshIncome);
  stopSyncListeners = [
    EventsOn("sync_applied:branch_targets", refreshIncome),
    EventsOn("sync_applied:currencies", refreshIncome),
    EventsOn("sync_applied:cashbox_accounts", refreshIncome),
  ];
  try {
    paymentMethods.value = ((await ListPaymentMethods()) as PaymentMethod[]).filter(
      (method) => !/^(on account|on coupon)$/i.test(method.name.trim()),
    );
  } catch {
    paymentMethods.value = [];
  }
  try {
    const settings = await GetAppSettings();
    openingFloatEnabled.value = settings?.openingFloatEnabled !== false;
    nonAdminMonthNavEnabled.value = Boolean(settings?.nonAdminMonthNavEnabled);
  } catch {}
  try {
    currentMonth.value = await GetCurrentMonth();
  } catch {}
  if (currentMonth.value === `${new Date().getFullYear()}-${pad(new Date().getMonth() + 1)}`) {
    expandedDays.value = [currentDay.value];
  }
  await refresh();
});

onBeforeUnmount(() => {
  window.removeEventListener(INCOME_CHANGED_EVENT, refreshIncome);
  stopSyncListeners.forEach((stop) => stop());
});
</script>

<style scoped>
.shifts-page {
  overflow-y: auto;
}

.shifts-cards {
  flex: 1 1 auto;
  min-height: 0;
}
</style>
