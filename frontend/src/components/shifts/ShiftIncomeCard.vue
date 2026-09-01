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
          <v-avatar :color="paymentAccentColor({ name: item.accountName, color: item.color })" variant="tonal" size="32">
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
      <div class="d-flex justify-space-between px-4 pt-3 mt-2 border-t font-weight-bold">
        <span>Income total</span>
        <span>{{ formatBalance(incomeTotal, "USD", currencies, roundingMode) }}</span>
      </div>
    </template>
    <div v-else class="shift-income-empty pa-3 text-caption text-medium-emphasis">
      <div class="shift-income-empty-message d-flex align-center justify-center">
        <v-icon icon="mdi-information-outline" size="18" class="me-1" />
        {{ shiftId ? "No income found for the current shift." : "No open shift found." }}
      </div>
    </div>
    <div v-if="shiftId" class="shift-income-actions mt-auto">
      <v-btn
        v-if="balanceSources.length > 0"
        block
        prepend-icon="mdi-swap-horizontal"
        variant="tonal"
        @click="replacementOpen = true"
      >
        Swap balance
      </v-btn>
      <v-btn
        v-if="openingFloatEnabled"
        block
        prepend-icon="mdi-cash-plus"
        variant="tonal"
        @click="openingFloatOpen = true"
      >
        Set opening cash
      </v-btn>
      <div
        v-if="openingFloatEnabled && (openingFloatUsd || openingFloatLbp)"
        class="shift-income-opening text-caption text-medium-emphasis px-1"
      >
        Opening amount: {{ formatUsd((openingFloatUsd ?? 0) / 100) }} / {{ Math.round(openingFloatLbp ?? 0).toLocaleString() }} LBP
      </div>
      <v-btn
        block
        color="primary"
        prepend-icon="mdi-cash-minus"
        variant="tonal"
        @click="spendOpen = true"
      >
        Spend
      </v-btn>
    </div>
  </v-sheet>
  <v-dialog v-model="spendOpen" max-width="520">
    <v-card rounded="xl">
      <div class="dialog-close-row px-4 pt-0">
        <v-btn
          icon="mdi-close"
          variant="text"
          aria-label="Close spend menu"
          @click="spendOpen = false"
        />
      </div>
      <v-card-text class="spend-options px-4 pt-0 pb-4">
        <v-card class="spend-option" rounded="lg" variant="tonal" @click="openExpenses">
          <v-icon icon="mdi-receipt-text-outline" size="44" />
          <span>Expenses</span>
        </v-card>
        <v-card class="spend-option" rounded="lg" variant="tonal" @click="openLoans">
          <v-icon icon="mdi-hand-coin-outline" size="44" />
          <span>Loans</span>
        </v-card>
      </v-card-text>
    </v-card>
  </v-dialog>
  <BalanceReplacementDialog v-model="replacementOpen" :shift-id="shiftId" :sources="balanceSources" :payment-methods="paymentMethods" @saved="emit('balance-replaced', $event)" />
  <SetOpeningFloatDialog
    v-if="openingFloatEnabled"
    v-model="openingFloatOpen"
    :shift-id="shiftId"
    :opening-float-usd-cents="openingFloatUsd"
    :opening-float-lbp="openingFloatLbp"
    @saved="emit('opening-float-set')"
  />
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { useRouter } from "vue-router";
import type { Balance, IncomeRow, PaymentMethod } from "@/types/shifts";
import { formatBalance, formatUsd } from "@/lib/format";
import { paymentAccentColor } from "@/lib/paymentColor";
import { useCurrencies } from "@/composables/useCurrencies";
import BalanceReplacementDialog from "@/components/shifts/BalanceReplacementDialog.vue";
import SetOpeningFloatDialog from "@/components/shifts/SetOpeningFloatDialog.vue";

const { currencies, roundingMode } = useCurrencies();
const router = useRouter();

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
  openingFloatEnabled?: boolean;
}>();
const emit = defineEmits<{
  "balance-replaced": [shiftId: number];
  "opening-float-set": [];
}>();
const replacementOpen = ref(false);
const openingFloatOpen = ref(false);
const spendOpen = ref(false);
const balanceSources = computed<Balance[]>(() => props.balanceSources
  .filter((item) => !item.isOnAccount && !item.isOnCoupon && item.accountId && Number(item.balance) !== 0));

const incomeTotal = computed(() => Number(props.incomeTotal) || 0);

function openExpenses() {
  spendOpen.value = false;
  void router.push("/expenses");
}

function openLoans() {
  spendOpen.value = false;
  void router.push("/loans");
}
</script>

<style scoped>
.shifts-card {
  height: 100%;
  min-height: 0;
}

.shift-income-card {
  display: flex;
  flex-direction: column;
}

.shift-income-title {
  display: flex;
  align-items: center;
  min-height: 40px;
}

.shift-income-empty {
  display: flex;
  align-items: center;
  flex-direction: column;
}

.shift-income-empty-message {
  width: 100%;
}

.shift-income-actions {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  gap: 8px;
}

/* Vuetify gives .v-btn--block `flex: 1 0 auto`; stop the buttons stretching
   to fill the card's free space so the action group stays compact. */
.shift-income-actions > .v-btn {
  flex: 0 0 auto;
}

.shift-income-opening {
  margin-top: -2px;
}

.dialog-close-row {
  display: flex;
  justify-content: flex-end;
}

.spend-options {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.spend-option {
  align-items: center;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  font-size: var(--kashi-font-size-lg);
  font-weight: 600;
  gap: 12px;
  justify-content: center;
  min-height: 144px;
  padding: 18px 12px;
  text-align: center;
  transition: background-color 0.15s ease;
}

.spend-option:hover {
  background-color: rgba(var(--v-theme-primary), 0.12);
}
</style>
