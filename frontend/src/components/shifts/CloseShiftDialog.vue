<template>
  <v-dialog
    :model-value="modelValue"
    max-width="420"
    @update:model-value="$emit('update:modelValue', $event)"
  >
    <v-card :title="`Close Shift #${openShift?.id}`">
      <v-card-text>
        <div v-if="closeBalances.length" class="mb-4">
          <div
            v-for="balance in closeBalances"
            :key="balance.accountId"
            class="d-flex justify-space-between py-1"
          >
            <span>{{ balance.accountName }}</span>
            <span class="font-weight-bold">${{ (balance.balance / 100).toFixed(2) }}</span>
          </div>
        </div>
        <div v-else class="text-center text-medium-emphasis mb-4">No transactions this shift</div>
      </v-card-text>
      <v-card-actions class="pa-2 ga-2">
        <v-btn variant="text" @click="$emit('update:modelValue', false)">Cancel</v-btn>
        <v-btn color="red-darken-4" variant="elevated" :loading="closing" @click="$emit('confirm')">
          Confirm Close
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import type { Balance, Shift } from "@/types/shifts";

defineProps<{
  modelValue: boolean;
  openShift: Shift | null;
  closeBalances: Balance[];
  closing: boolean;
}>();

defineEmits<{
  "update:modelValue": [value: boolean];
  confirm: [];
}>();
</script>
