<template>
  <div class="d-flex ga-4 pa-4" style="width: 100%; overflow: hidden">
    <v-card class="pa-4" style="flex: 1; min-width: 0">
      <v-card-title class="text-body-1 font-weight-bold">Today's Income</v-card-title>
      <v-data-table :headers="todayHeaders" :items="todayIncome" :loading="loadingToday" density="compact" hide-default-footer />
    </v-card>

    <v-card class="pa-4" style="flex: 2; min-width: 0">
      <v-card-title class="text-body-1 font-weight-bold">Monthly Income ({{ currentMonth }})</v-card-title>
      <v-data-table :headers="monthHeaders" :items="monthlyIncome" :loading="loadingMonth" density="compact" hide-default-footer>
        <template #item.total="{ item }">${{ (item.total / 100).toFixed(2) }}</template>
      </v-data-table>
    </v-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { GetCurrentMonth, GetDailyIncomeByMonth, GetTodayIncomeByAccount } from "../../wailsjs/go/main/App";

const todayHeaders = [
  { title: "Account", key: "accountName" },
  { title: "Total", key: "total" },
];
const monthHeaders = [
  { title: "Day", key: "day" },
  { title: "Total", key: "total" },
];

const todayIncome = ref<any[]>([]);
const monthlyIncome = ref<any[]>([]);
const currentMonth = ref("");
const loadingToday = ref(true);
const loadingMonth = ref(true);

onMounted(async () => {
  try { currentMonth.value = await GetCurrentMonth(); } catch {}
  try { todayIncome.value = await GetTodayIncomeByAccount(); } catch {}
  try { monthlyIncome.value = await GetDailyIncomeByMonth(currentMonth.value || new Date().toISOString().slice(0, 7)); } catch {}
  loadingToday.value = false;
  loadingMonth.value = false;
});
</script>
