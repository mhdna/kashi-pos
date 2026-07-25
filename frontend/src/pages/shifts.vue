<template>
  <div class="shifts-page d-flex flex-column ga-2 pa-2 h-100">
    <ShiftSummaryCard :open-shift="openShift" @close="closeDialog = true" />

    <div class="shifts-cards d-flex ga-2">
      <TodayIncomeCard :items="todayIncome" :loading="loadingToday" />
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
        @toggle-all="toggleAllDays"
        @toggle-day="toggleDay"
        @toggle-shift="toggleShift"
      />
    </div>

    <CloseShiftDialog
      v-model="closeDialog"
      :open-shift="openShift"
      :close-balances="closeBalances"
      :closing="closing"
      @confirm="doCloseShift"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import {
  CloseShiftByID,
  GetCurrentMonth,
  GetOpenShift,
  GetShiftTotals,
  GetTodayIncomeByAccount,
  ListShiftBalances,
  OpenShift,
} from "../../wailsjs/go/main/App";
import CloseShiftDialog from "@/components/shifts/CloseShiftDialog.vue";
import MonthlyIncomeCard from "@/components/shifts/MonthlyIncomeCard.vue";
import ShiftSummaryCard from "@/components/shifts/ShiftSummaryCard.vue";
import TodayIncomeCard from "@/components/shifts/TodayIncomeCard.vue";
import type { Balance, DayRow, IncomeRow, Shift } from "@/types/shifts";

const openShift = ref<Shift | null>(null);
const todayIncome = ref<IncomeRow[]>([]);
const allShifts = ref<Shift[]>([]);
const currentMonth = ref("");
const loadingToday = ref(true);
const loadingMonth = ref(true);

const balanceCache = reactive<Record<number, Balance[]>>({});
const balanceLoading = reactive<Record<number, boolean>>({});
const shiftExpanded = reactive(new Set<number>());
const expandedDays = ref<string[]>([]);

const dailyRows = computed<DayRow[]>(() => {
  const groups: Record<string, Shift[]> = {};
  for (const shift of allShifts.value) {
    const day = shift.createdAt?.split(" ")[0] ?? "—";
    groups[day] ??= [];
    groups[day].push(shift);
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
const allDaysExpanded = computed(
  () =>
    dailyRows.value.length > 0 &&
    dailyRows.value.every((row) => expandedDays.value.includes(row.day)),
);

function pad(value: number) {
  return value.toString().padStart(2, "0");
}

function dayTotal(shifts: Shift[]) {
  return shifts.reduce((total, shift) => total + shift.total, 0);
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
const closing = ref(false);

async function doCloseShift() {
  if (!openShift.value) return;

  closing.value = true;
  try {
    await CloseShiftByID(openShift.value.id, closeBalances.value);
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
  loadingToday.value = true;
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
    allShifts.value = ((await GetShiftTotals()) as Shift[]) ?? [];
  } catch {
    allShifts.value = [];
  }

  try {
    todayIncome.value = ((await GetTodayIncomeByAccount()) as IncomeRow[]) ?? [];
  } catch {
    todayIncome.value = [];
  }

  loadingToday.value = false;
  loadingMonth.value = false;
}

onMounted(async () => {
  try {
    currentMonth.value = await GetCurrentMonth();
  } catch {}
  await refresh();
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
