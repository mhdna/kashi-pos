<template>
  <v-sheet class="shift-income-card shifts-card pa-4" style="flex: 1; min-width: 0">
    <v-card-title class="shift-income-title text-body-1 font-weight-bold px-0">
      <v-icon icon="mdi-cash-register" color="grey-darken-1" class="me-2" />
      {{ title || "Shift Income" }}{{ shiftId ? ` (#${shiftId})` : "" }}
    </v-card-title>
    <v-progress-linear v-if="loading" indeterminate />
    <template v-else-if="items.length">
      <v-list density="compact" class="pa-0">
        <v-list-item v-for="item in items" :key="item.accountName">
        <template #prepend>
          <v-avatar :color="incomeColor(item.accountName)" variant="tonal" size="32">
            <v-icon icon="mdi-wallet" size="18" />
          </v-avatar>
        </template>
        <template #title>
          {{ item.isOnAccount ? "(On Account) — not collected" : item.accountName }}
        </template>
        <template #append>
          <span class="font-weight-bold text-medium-emphasis">
            {{ formatBalance(item.total, item.currencyCode ?? "USD", currencies, roundingMode) }}
          </span>
        </template>
        </v-list-item>
      </v-list>
      <v-btn
        v-if="shiftId && balanceSources.length > 0"
        block
        class="mt-3"
        prepend-icon="mdi-swap-horizontal"
        variant="tonal"
        @click="replacementOpen = true"
      >
        Swap balance
      </v-btn>
      <v-btn
        v-if="shiftId"
        block
        class="mt-2"
        prepend-icon="mdi-cash-plus"
        variant="tonal"
        @click="openingFloatOpen = true"
      >
        Set opening cash
      </v-btn>
      <div v-if="shiftId && (openingFloatUsd || openingFloatLbp)" class="text-caption text-medium-emphasis px-1 pt-1">
        Opening float: {{ formatUsd((openingFloatUsd ?? 0) / 100) }} / {{ Math.round(openingFloatLbp ?? 0).toLocaleString() }} LBP
      </div>
      <div class="d-flex justify-space-between px-4 pt-3 mt-2 border-t font-weight-bold">
        <span>Income total</span>
        <span>{{ formatBalance(incomeTotal, "USD", currencies, roundingMode) }}</span>
      </div>
    </template>
    <div v-else class="shift-income-empty pa-3 text-caption text-medium-emphasis">
      <v-icon icon="mdi-information-outline" size="18" class="me-1" />
      {{ shiftId ? "No income found for the current shift." : "No open shift found." }}
    </div>
  </v-sheet>
  <BalanceReplacementDialog v-model="replacementOpen" :shift-id="shiftId" :sources="balanceSources" :payment-methods="paymentMethods" @saved="emit('balance-replaced', $event)" />
  <SetOpeningFloatDialog
    v-model="openingFloatOpen"
    :shift-id="shiftId"
    :opening-float-usd-cents="openingFloatUsd"
    :opening-float-lbp="openingFloatLbp"
    @saved="emit('opening-float-set')"
  />
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import type { Balance, IncomeRow, PaymentMethod } from "@/types/shifts";
import { formatBalance, formatUsd } from "@/lib/format";
import { paymentColor } from "@/lib/paymentColor";
import { useCurrencies } from "@/composables/useCurrencies";
import BalanceReplacementDialog from "@/components/shifts/BalanceReplacementDialog.vue";
import SetOpeningFloatDialog from "@/components/shifts/SetOpeningFloatDialog.vue";

const { currencies, roundingMode } = useCurrencies();

const props = defineProps<{
  items: IncomeRow[];
  title?: string;
  shiftId?: number;
  incomeTotal?: number;
  paymentMethods: PaymentMethod[];
  balanceSources: Balance[];
  loading: boolean;
  openingFloatUsd?: number;
  openingFloatLbp?: number;
}>();
const emit = defineEmits<{ "balance-replaced": [shiftId: number]; "opening-float-set": [] }>();
const replacementOpen = ref(false);
const openingFloatOpen = ref(false);
const balanceSources = computed<Balance[]>(() => props.balanceSources
  .filter((item) => !item.isOnAccount && !item.isOnCoupon && item.accountId && Number(item.balance) !== 0));

const incomeTotal = computed(() => Number(props.incomeTotal) || 0);

function incomeColor(name: string): string {
  return paymentColor(name);
}
</script>

<style scoped>
.shifts-card {
  height: 100%;
  min-height: 0;
}

.shift-income-title {
  display: flex;
  align-items: center;
  min-height: 40px;
}

.shift-income-empty {
  display: flex;
  align-items: center;
}
</style>
