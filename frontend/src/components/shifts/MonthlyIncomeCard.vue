<template>
  <v-card class="monthly-income-card shifts-card pa-4" style="flex: 2; min-width: 0">
    <div class="monthly-income-title d-flex align-center">
      <v-card-title class="text-body-1 font-weight-bold px-0">
        Monthly Income ({{ month }})
      </v-card-title>
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
    <v-virtual-scroll v-else :items="rows" :item-height="40" class="monthly-income-scroll">
      <template #default="{ item }">
        <div>
          <div class="monthly-income-row">
            <v-btn
              class="monthly-income-expand"
              variant="text"
              density="compact"
              size="small"
              :icon="expandedDays.includes(item.day) ? 'mdi-chevron-up' : 'mdi-chevron-down'"
              :aria-label="`${expandedDays.includes(item.day) ? 'Collapse' : 'Expand'} ${item.day}`"
              @click="$emit('toggle-day', item.day)"
            />
            <span>{{ item.day }}</span>
            <span class="font-weight-bold">${{ (item.total / 100).toFixed(2) }}</span>
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
                  <span class="text-caption me-2">{{ shift.createdAt }}</span>
                  <span v-if="shift.closedAt" class="text-caption me-2"
                    >→ {{ shift.closedAt }}</span
                  >
                  <v-spacer />
                  <span class="font-weight-bold me-2">${{ (shift.total / 100).toFixed(2) }}</span>
                  <v-icon
                    :icon="shiftExpanded.has(shift.id) ? 'mdi-chevron-up' : 'mdi-chevron-down'"
                    size="16"
                  />
                </div>
                <div v-if="shiftExpanded.has(shift.id)" class="pa-3">
                  <div class="text-caption font-weight-bold mb-1">Balances</div>
                  <v-progress-linear v-if="balanceLoading[shift.id]" indeterminate />
                  <v-list v-else-if="balanceCache[shift.id]?.length" density="compact" class="pa-0">
                    <v-list-item v-for="balance in balanceCache[shift.id]" :key="balance.accountId">
                      <template #title>{{ balance.accountName }}</template>
                      <template #append>
                        <span class="font-weight-bold"
                          >${{ (balance.balance / 100).toFixed(2) }}</span
                        >
                      </template>
                    </v-list-item>
                  </v-list>
                  <div v-else class="text-caption text-medium-emphasis">No balance data found.</div>
                </div>
              </div>
            </template>
          </div>
        </div>
      </template>
    </v-virtual-scroll>
    <div class="monthly-income-total font-weight-bold">
      <span>Total</span>
      <span>${{ (grandTotal / 100).toFixed(2) }}</span>
    </div>
  </v-card>
</template>

<script setup lang="ts">
import type { Balance, DayRow } from "@/types/shifts";

defineProps<{
  month: string;
  rows: DayRow[];
  grandTotal: number;
  loading: boolean;
  expandedDays: string[];
  allDaysExpanded: boolean;
  shiftExpanded: Set<number>;
  balanceCache: Record<number, Balance[]>;
  balanceLoading: Record<number, boolean>;
}>();

defineEmits<{
  "toggle-all": [];
  "toggle-day": [day: string];
  "toggle-shift": [shiftId: number];
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
}

.monthly-income-title {
  min-height: 40px;
}

.monthly-income-scroll {
  min-height: 0;
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
}

.monthly-income-row {
  min-height: 40px;
  padding: 0 12px 0 0;
  border-bottom: 1px solid rgba(128, 128, 128, 0.2);
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
}
</style>
