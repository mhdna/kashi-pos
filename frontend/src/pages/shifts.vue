<template>
  <div class="shifts-page d-flex flex-column h-100">
    <!-- <ShiftSummaryCard :open-shift="openShift" @close="closeDialog = true" /> -->

    <div class="shifts-cards d-flex">
      <ShiftIncomeCard
        :items="shiftIncome"
        title="Today's Income"
        :loading="loadingShift"
        :exchange-rate="exchangeRate"
      />
      <MonthlyIncomeCard
        :month="currentMonth"
        :rows="dailyRows"
        :grand-total="grandTotal"
        :loading="loadingMonth"
        :expanded-days="expandedDays"
        :all-days-expanded="allDaysExpanded"
        :shift-expanded="shiftExpanded"
        :balance-cache="balanceCache"
        :balance-loading="balanceLoading"
        :exchange-rate="exchangeRate"
        :current-day="currentDay"
        :is-admin="auth.isAdmin"
        :printing-shift-id="printingShiftId"
        @toggle-all="toggleAllDays"
        @toggle-day="toggleDay"
        @toggle-shift="toggleShift"
        @print-shift="printClosedShift"
      />
    </div>

    <CloseShiftDialog
      v-model="closeDialog"
      :open-shift="openShift"
      :close-balances="closeBalances"
      :salespersons="salespersons"
      :closing="closing"
      :exchange-rate="exchangeRate"
      @confirm="doCloseShift"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import {
  CloseShiftByID,
  GetDailyIncomeByMonth,
  GetCurrentMonth,
  GetOpenShift,
  GetShiftTotals,
  GetTodayIncomeByAccount,
  GetAppSettings,
  ListShiftBalances,
  ListSalespersons,
  OpenShift,
  PrintClosedShiftReceipt,
} from "../../wailsjs/go/main/App";
import CloseShiftDialog from "@/components/shifts/CloseShiftDialog.vue";
import MonthlyIncomeCard from "@/components/shifts/MonthlyIncomeCard.vue";
import ShiftSummaryCard from "@/components/shifts/ShiftSummaryCard.vue";
import ShiftIncomeCard from "@/components/shifts/ShiftIncomeCard.vue";
import { INCOME_CHANGED_EVENT } from "@/lib/incomeEvents";
import type { Balance, DayRow, IncomeRow, Shift } from "@/types/shifts";
import { useAuthStore } from "@/stores/auth";
const auth = useAuthStore();
const exchangeRate = ref(90000);
const salespersons = ref<Array<{ id: number; fullName: string }>>([]);

const openShift = ref<Shift | null>(null);
const allShifts = ref<Shift[]>([]);
const currentMonth = ref("");
const dailyIncomeByDay = ref<Record<string, number>>({});
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
const shiftIncome = computed<IncomeRow[]>(() =>
  todayIncome.value.map((income) => ({
    accountName: income.accountName,
    currencyCode: income.currencyCode,
    total: Number(income.total) || 0,
    isOnAccount: income.isOnAccount,
    isCollected: income.isCollected,
  })),
);

const dailyRows = computed<DayRow[]>(() => {
  const groups: Record<string, Shift[]> = {};
  for (const shift of allShifts.value) {
    const day = shiftDayKey(shift.isClosed && shift.closedAt ? shift.closedAt : shift.createdAt);
    groups[day] ??= [];
    groups[day].push(shift);
  }

  // An open shift can continue past midnight. Keep it visible under today,
  // even when it was opened on the previous day and has no income yet.
  const activeShift = openShift.value;
  if (activeShift?.id) {
    const activeShiftTotal = allShifts.value.find((shift) => shift.id === activeShift.id)?.total;
    for (const shifts of Object.values(groups)) {
      const index = shifts.findIndex((shift) => shift.id === activeShift.id);
      if (index >= 0) shifts.splice(index, 1);
    }
    const todayShifts = (groups[currentDay.value] ??= []);
    todayShifts.push({
      ...activeShift,
      total: Number.isFinite(Number(activeShiftTotal)) ? Number(activeShiftTotal) : 0,
    });
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
    const isoDay = `${year}-${month}-${pad(index + 1)}`;
    return { day, total: dailyIncomeByDay.value[isoDay] ?? 0, shifts };
  });
});

const grandTotal = computed(() => dailyRows.value.reduce((total, row) => total + row.total, 0));
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

function shiftDayKey(value?: string) {
  const date = value?.split(" ")[0] ?? "";
  const isoMatch = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date);
  if (isoMatch) return `${isoMatch[3]}/${isoMatch[2]}/${isoMatch[1]}`;
  return date || "—";
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

const closeDialog = ref(false);
const closeBalances = ref<Balance[]>([]);
const todayIncome = ref<IncomeRow[]>([]);
const closing = ref(false);

async function doCloseShift(
  closingPersonId: number,
  countedLbp: number,
  countedVisa: number,
  countedWhish: number,
  usdBreakdownJson: string,
) {
  if (!openShift.value) return;

  closing.value = true;
  try {
    const result = await CloseShiftByID(
      openShift.value.id,
      closingPersonId,
      countedLbp,
      countedVisa,
      countedWhish,
      usdBreakdownJson,
    );
    if (result?.receiptError) {
      window.alert(`Shift closed, but the receipt could not be printed: ${result.receiptError}`);
    }
    await OpenShift();
    closeDialog.value = false;
    await refresh();
  } catch (error) {
    console.error("close shift:", error);
  } finally {
    closing.value = false;
  }
}

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
    todayIncome.value = ((await GetTodayIncomeByAccount()) as IncomeRow[]) ?? [];
  } catch {
    todayIncome.value = [];
  }

  try {
    const shifts = ((await GetShiftTotals()) as Shift[]) ?? [];
    allShifts.value = shifts.map((shift) => ({
      ...shift,
      total: Number.isFinite(Number(shift.total)) ? Number(shift.total) : 0,
    }));
  } catch {
    allShifts.value = [];
  }

  try {
    const rows =
      ((await GetDailyIncomeByMonth(currentMonth.value)) as Array<{
        day: string;
        total: number;
      }>) ?? [];
    dailyIncomeByDay.value = Object.fromEntries(
      rows.map((row) => [row.day, Number(row.total) || 0]),
    );
  } catch {
    dailyIncomeByDay.value = {};
  }

  for (const shiftId of Object.keys(balanceCache)) delete balanceCache[Number(shiftId)];
  await Promise.all([...shiftExpanded].map(loadBalances));

  loadingShift.value = false;
  loadingMonth.value = false;
}

function refreshIncome() {
  void refresh();
}

onMounted(async () => {
  window.addEventListener(INCOME_CHANGED_EVENT, refreshIncome);
  try {
    const settings = await GetAppSettings();
    const rate = Number(settings?.exchangeRate);
    if (Number.isFinite(rate) && rate > 0) exchangeRate.value = rate;
  } catch {}
  try {
    salespersons.value =
      ((await ListSalespersons()) as Array<{ id: number; fullName: string }>) ?? [];
  } catch {
    salespersons.value = [];
  }
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
