<template>
  <v-dialog
    :model-value="modelValue"
    max-width="520"
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
            <span class="font-weight-bold">{{ formatBalance(balance.balance, balance.currencyCode ?? "USD", exchangeRate ?? 90000) }}</span>
          </div>
        </div>
        <div v-else class="text-center text-medium-emphasis mb-4">No transactions this shift</div>

        <v-select
          v-model="closingPersonId"
          :items="salespersons"
          item-title="fullName"
          item-value="id"
          placeholder="Closing person"
          variant="outlined"
          density="comfortable"
          class="mt-3"
        />

        <div class="text-subtitle-2 mb-2">USD bill count</div>
        <div class="usd-bill-grid">
          <v-text-field
            v-for="denomination in usdDenominations"
            :key="denomination"
            v-model="usdBillCounts[denomination]"
            :placeholder="`$${denomination} bills`"
            type="number"
            min="0"
            step="1"
            variant="outlined"
            density="compact"
            :prepend-inner-icon="denomination === 100 ? 'mdi-cash-100' : 'mdi-cash'"
          />
        </div>
        <div class="d-flex justify-space-between align-center mb-4 px-1">
          <span class="text-body-2">USD counted total</span>
          <span class="text-subtitle-1 font-weight-bold">{{ formatUsd(countedUsdTotal / 100) }}</span>
        </div>
        <v-text-field
          v-model="countedLbp"
          placeholder="Total LBP counted"
          prepend-inner-icon="mdi-cash"
          suffix="LBP"
          type="number"
          min="0"
          step="1000"
          variant="outlined"
          density="comfortable"
        />
        <div class="d-flex justify-space-between align-center mb-2 px-1">
          <span class="text-body-2">LBP counted total</span>
          <span class="text-subtitle-1 font-weight-bold">{{ countedLbpTotal }}</span>
        </div>
      </v-card-text>
      <v-card-actions class="pa-2 ga-2">
        <v-btn variant="text" @click="$emit('update:modelValue', false)">Cancel</v-btn>
        <v-btn
          color="red-darken-4"
          variant="elevated"
          :loading="closing"
          :disabled="!formValid"
          @click="confirmClose"
        >
          Close & Print
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import type { Balance, Shift } from "@/types/shifts";
import { formatBalance, formatUsd } from "@/lib/format";

const props = defineProps<{
  modelValue: boolean;
  openShift: Shift | null;
  closeBalances: Balance[];
  salespersons: Array<{ id: number; fullName: string }>;
  closing: boolean;
  exchangeRate?: number;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: boolean];
  confirm: [closingPersonId: number, countedLbp: number, usdBreakdownJson: string];
}>();

const closingPersonId = ref<number | null>(null);
const countedLbp = ref("");
const usdDenominations = [100, 50, 20, 10, 5, 1] as const;
const usdBillCounts = ref<Record<number, string>>(
  Object.fromEntries(usdDenominations.map((denomination) => [denomination, ""])),
);

const parsedLbp = computed(() => Number(countedLbp.value));
const billCountsValid = computed(() =>
  usdDenominations.every((denomination) => {
    const raw = usdBillCounts.value[denomination] ?? "";
    if (raw === "") return true;
    const count = Number(raw);
    return Number.isInteger(count) && count >= 0;
  }),
);
const countedUsdTotal = computed(() =>
  usdDenominations.reduce(
    (total, denomination) =>
      total + denomination * Number(usdBillCounts.value[denomination] || 0) * 100,
    0,
  ),
);
const countedLbpTotal = computed(
  () => `${Math.round(Number(countedLbp.value || 0)).toLocaleString()} LBP`,
);
const formValid = computed(
  () =>
    Boolean(closingPersonId.value) &&
    countedLbp.value !== "" &&
    Number.isFinite(parsedLbp.value) &&
    parsedLbp.value >= 0 &&
    billCountsValid.value,
);

watch(
  () => props.modelValue,
  (open) => {
    if (!open) return;
    closingPersonId.value = null;
    countedLbp.value = "";
    for (const denomination of usdDenominations) usdBillCounts.value[denomination] = "";
  },
);

function confirmClose() {
  if (!formValid.value || !closingPersonId.value) return;
  emit(
    "confirm",
    closingPersonId.value,
    Math.round(parsedLbp.value),
    JSON.stringify(
      Object.fromEntries([
        ...usdDenominations.map((denomination) => [
          denomination.toString(),
          Number(usdBillCounts.value[denomination] || 0),
        ]),
      ]),
    ),
  );
}

</script>

<style scoped>
.usd-bill-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0 12px;
}

</style>
