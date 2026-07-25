<template>
  <v-card class="shifts-card pa-4" style="flex: 1; min-width: 0">
    <v-card-title class="text-body-1 font-weight-bold">Today's Income</v-card-title>
    <v-progress-linear v-if="loading" indeterminate />
    <v-list v-else-if="items.length" density="compact" class="pa-0">
      <v-list-item v-for="item in items" :key="item.accountName">
        <template #title>{{ item.accountName }}</template>
        <template #append>
          <span class="font-weight-bold">${{ (item.total / 100).toFixed(2) }}</span>
        </template>
      </v-list-item>
    </v-list>
    <div v-else class="pa-3 text-caption text-medium-emphasis">No income data found for today.</div>
  </v-card>
</template>

<script setup lang="ts">
import type { IncomeRow } from "@/types/shifts";

defineProps<{
  items: IncomeRow[];
  loading: boolean;
}>();
</script>

<style scoped>
.shifts-card {
  height: 100%;
  min-height: 0;
}
</style>
