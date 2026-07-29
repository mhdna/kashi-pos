<template>
  <v-card class="today-income-card shifts-card pa-4" style="flex: 1; min-width: 0">
    <v-card-title class="today-income-title text-body-1 font-weight-bold px-0">
      <v-icon icon="mdi-cash-register" color="primary" class="me-2" />
      Today's Income
    </v-card-title>
    <v-progress-linear v-if="loading" indeterminate />
    <v-list v-else-if="items.length" density="compact" class="pa-0">
      <v-list-item v-for="item in items" :key="item.accountName">
        <template #prepend>
          <v-avatar color="primary" variant="tonal" size="32">
            <v-icon icon="mdi-wallet" size="18" />
          </v-avatar>
        </template>
        <template #title>{{ item.accountName }}</template>
        <template #append>
          <span class="font-weight-bold text-primary">
            {{ formatBalance(item.total, item.currencyCode ?? "USD", exchangeRate) }}
          </span>
        </template>
      </v-list-item>
    </v-list>
    <div v-else class="today-income-empty pa-3 text-caption text-medium-emphasis">
      <v-icon icon="mdi-information-outline" size="18" class="me-1" />
      No income data found for today.
    </div>
  </v-card>
</template>

<script setup lang="ts">
import type { IncomeRow } from "@/types/shifts";
import { formatBalance } from "@/lib/format";

defineProps<{
  items: IncomeRow[];
  loading: boolean;
  exchangeRate: number;
}>();
</script>

<style scoped>
.shifts-card {
  height: 100%;
  min-height: 0;
}

.today-income-title {
  display: flex;
  align-items: center;
  min-height: 40px;
}

.today-income-empty {
  display: flex;
  align-items: center;
}
</style>
