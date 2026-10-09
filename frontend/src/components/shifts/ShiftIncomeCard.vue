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
    <!-- Money that went out during the shift, then income minus it. Display
         only: the shift's expected cash still doesn't take expenses or loans
         out (see GetShiftTotals). -->
    <div v-if="shiftId && (shiftExpenses.length || shiftLoans.length)" class="px-4 pt-2">
      <div v-for="row in shiftExpenses" :key="`e-${row.id}`" class="spend-row">
        <span class="text-truncate"><v-icon icon="mdi-receipt-text-outline" size="16" class="me-1" />{{ row.description || row.category || "Expense" }}</span>
        <span class="text-error">−{{ formatSpendAmount(row.amount, row.currencyCode, currencies) }}</span>
      </div>
      <div v-for="row in shiftLoans" :key="`l-${row.id}`" class="spend-row">
        <span class="text-truncate"><v-icon icon="mdi-hand-coin-outline" size="16" class="me-1" />{{ row.description || row.category || "Loan" }}</span>
        <span class="text-error">−{{ formatSpendAmount(row.amount, row.currencyCode, currencies) }}</span>
      </div>
      <div class="d-flex justify-space-between pt-2 mt-1 border-t font-weight-bold">
        <span>Net income</span>
        <span>{{ formatBalance(netIncome, "USD", currencies, roundingMode) }}</span>
      </div>
    </div>
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
      <!-- Straight to each page, with this month's total underneath. -->
      <div class="spend-buttons">
        <v-btn class="spend-button" color="primary" variant="tonal" @click="openExpenses">
          <div class="spend-button-body">
            <span class="spend-button-label"><v-icon icon="mdi-receipt-text-outline" size="18" class="me-1" />Expenses</span>
            <span class="spend-button-total">{{ formatCurrencyTotals(monthExpenses, currencies) }} this month</span>
          </div>
        </v-btn>
        <v-btn class="spend-button" color="primary" variant="tonal" @click="openLoans">
          <div class="spend-button-body">
            <span class="spend-button-label"><v-icon icon="mdi-hand-coin-outline" size="18" class="me-1" />Loans</span>
            <span class="spend-button-total">{{ formatCurrencyTotals(monthLoans, currencies) }} this month</span>
          </div>
        </v-btn>
      </div>
    </div>
  </v-sheet>
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
import { computed, onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { GetSpendSummary } from "../../../wailsjs/go/main/App";
import { formatCurrencyTotals, formatSpendAmount, sumToUsdCents, type CurrencyTotal } from "@/lib/spend";
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

const monthExpenses = ref<CurrencyTotal[]>([]);
const monthLoans = ref<CurrencyTotal[]>([]);
const shiftExpenses = ref<any[]>([]);
const shiftLoans = ref<any[]>([]);
async function loadSpend() {
  try {
    const summary = await GetSpendSummary(props.shiftId ?? 0);
    monthExpenses.value = summary.monthExpenses ?? [];
    monthLoans.value = summary.monthLoans ?? [];
    shiftExpenses.value = summary.shiftExpenses ?? [];
    shiftLoans.value = summary.shiftLoans ?? [];
  } catch {
    // Totals just stay as they were; the buttons still work.
  }
}
onMounted(loadSpend);
watch(() => props.shiftId, loadSpend);
const netIncome = computed(
  () =>
    (Number(props.incomeTotal) || 0) -
    sumToUsdCents(shiftExpenses.value, currencies.value) -
    sumToUsdCents(shiftLoans.value, currencies.value),
);
const balanceSources = computed<Balance[]>(() => props.balanceSources
  .filter((item) => !item.isOnAccount && !item.isOnCoupon && item.accountId && Number(item.balance) !== 0));

const incomeTotal = computed(() => Number(props.incomeTotal) || 0);

function openExpenses() {
  void router.push("/expenses");
}

function openLoans() {
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

.spend-row {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  font-size: 0.85rem;
  padding: 2px 0;
}

.spend-buttons {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.spend-button {
  height: auto !important;
  min-height: 56px;
  padding: 8px 6px !important;
}

.spend-button-body {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  white-space: normal;
}

.spend-button-label {
  display: inline-flex;
  align-items: center;
  font-weight: 700;
}

.spend-button-total {
  font-size: 0.75rem;
  font-weight: 500;
  opacity: 0.85;
  text-transform: none;
  letter-spacing: 0;
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
