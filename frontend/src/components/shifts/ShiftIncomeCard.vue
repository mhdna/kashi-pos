<template>
  <k-sheet class="shift-income-card shifts-card pa-4" style="flex: 1; min-width: 0">
    <k-card-title class="shift-income-title text-body-1 font-weight-bold px-0">
      <k-icon icon="mdi-cash-register" color="grey-darken-1" class="me-2" />
      {{ title || "Shift Income" }}{{ shiftId ? ` (#${shiftId})` : "" }}
    </k-card-title>
    <k-progress-linear v-if="loading" indeterminate />
    <template v-else-if="items.length">
      <k-list density="compact" class="pa-0">
        <k-list-item v-for="item in collectedItems" :key="item.accountName">
        <template #prepend>
          <k-avatar :color="paymentAccentColor({ name: item.accountName, color: item.color })" variant="tonal" size="32">
            <k-icon icon="mdi-wallet" size="18" />
          </k-avatar>
        </template>
        <template #title>
          {{ item.accountName }}
        </template>
        <template #append>
          <span class="font-weight-bold text-medium-emphasis">
            {{ formatBalance(item.total, item.currencyCode ?? "USD", currencies, roundingMode) }}
          </span>
        </template>
        </k-list-item>
      </k-list>
      <div class="d-flex justify-space-between px-4 pt-3 mt-2 border-t font-weight-bold">
        <span>Income total</span>
        <span>{{ formatBalance(collectedTotal, "USD", currencies, roundingMode) }}</span>
      </div>
    </template>
    <div v-else class="shift-income-empty pa-3 text-caption text-medium-emphasis">
      <div class="shift-income-empty-message d-flex align-center justify-center">
        <k-icon icon="mdi-information-outline" size="18" class="me-1" />
        {{ shiftId ? "No income found for the current shift." : "No open shift found." }}
      </div>
    </div>
    <!-- Sold on account (not collected, so it's kept out of the income total
         above), then money that went out during the shift, then income minus
         that. Display only (see GetShiftTotals). -->
    <div v-if="shiftId && (onAccountTotal || shiftExpenses.length || shiftLoans.length)" class="px-4 pt-2">
      <div v-if="onAccountTotal" class="spend-row text-medium-emphasis">
        <span class="text-truncate"><k-icon icon="mdi-account-clock" size="16" class="me-1" />On account — not collected</span>
        <span>{{ formatBalance(onAccountTotal, "USD", currencies, roundingMode) }}</span>
      </div>
      <div v-for="row in shiftExpenses" :key="`e-${row.id}`" class="spend-row">
        <span class="text-truncate"><k-icon icon="mdi-receipt-text-outline" size="16" class="me-1" />{{ row.description || row.category || "Expense" }}</span>
        <span class="text-error">−{{ formatSpendAmount(row.amount, row.currencyCode, currencies) }}</span>
      </div>
      <div v-for="row in shiftLoans" :key="`l-${row.id}`" class="spend-row">
        <span class="text-truncate"><k-icon icon="mdi-hand-coin-outline" size="16" class="me-1" />{{ row.description || row.category || "Loan" }}</span>
        <span class="text-error">−{{ formatSpendAmount(row.amount, row.currencyCode, currencies) }}</span>
      </div>
      <!-- Only when something came off; otherwise it just repeats the total. -->
      <div v-if="shiftExpenses.length || shiftLoans.length" class="d-flex justify-space-between pt-2 mt-1 border-t font-weight-bold">
        <span>Net income</span>
        <span>{{ formatBalance(netIncome, "USD", currencies, roundingMode) }}</span>
      </div>
    </div>
    <div v-if="shiftId" class="shift-income-actions mt-auto">
      <k-btn
        v-if="balanceSources.length > 0"
        block
        prepend-icon="mdi-swap-horizontal"
        variant="tonal"
        @click="replacementOpen = true"
      >
        Swap balance
      </k-btn>
      <k-btn
        v-if="openingFloatEnabled"
        block
        prepend-icon="mdi-cash-plus"
        variant="tonal"
        @click="openingFloatOpen = true"
      >
        Set opening cash
      </k-btn>
      <div
        v-if="openingFloatEnabled && (openingFloatUsd || openingFloatLbp)"
        class="shift-income-opening text-caption text-medium-emphasis px-1"
      >
        Opening amount: {{ formatUsd((openingFloatUsd ?? 0) / 100) }} / {{ Math.round(openingFloatLbp ?? 0).toLocaleString() }} LBP
      </div>
      <!-- Manager only: each asks for the manager password before opening. -->
      <div class="spend-buttons">
        <k-btn class="spend-button" color="primary" variant="tonal" @click="openManagerPage('/expenses')">
          <span class="spend-button-label"><k-icon icon="mdi-receipt-text-outline" size="18" class="me-1" />Expenses</span>
        </k-btn>
        <k-btn class="spend-button" color="primary" variant="tonal" @click="openManagerPage('/loans')">
          <span class="spend-button-label"><k-icon icon="mdi-hand-coin-outline" size="18" class="me-1" />Loans</span>
        </k-btn>
      </div>
    </div>
  </k-sheet>
  <BalanceReplacementDialog v-model="replacementOpen" :shift-id="shiftId" :sources="balanceSources" :payment-methods="paymentMethods" @saved="emit('balance-replaced', $event)" />
  <SetOpeningFloatDialog
    v-if="openingFloatEnabled"
    v-model="openingFloatOpen"
    :shift-id="shiftId"
    :opening-float-usd-cents="openingFloatUsd"
    :opening-float-lbp="openingFloatLbp"
    @saved="emit('opening-float-set')"
  />
  <k-dialog v-model="managerDialog.show" max-width="380">
    <k-card>
      <k-card-title class="d-flex align-center ga-2"><k-icon icon="mdi-lock-outline" />Manager password</k-card-title>
      <k-card-text>
        <k-text-field
          v-model="managerDialog.password"
          autofocus
          placeholder="Manager password"
          :type="managerDialog.reveal ? 'text' : 'password'"
          variant="outlined"
          density="compact"
          :append-inner-icon="managerDialog.reveal ? 'mdi-eye-off' : 'mdi-eye'"
          :error-messages="managerDialog.error"
          @click:append-inner="managerDialog.reveal = !managerDialog.reveal"
          @keyup.enter="confirmManager"
        />
      </k-card-text>
      <k-card-actions>
        <k-btn variant="text" @click="managerDialog.show = false">Cancel</k-btn><k-spacer />
        <k-btn color="primary" :loading="managerDialog.checking" @click="confirmManager">Open</k-btn>
      </k-card-actions>
    </k-card>
  </k-dialog>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { GetSpendSummary, VerifyManagerPassword } from "../../../wailsjs/go/main/App";
import { formatSpendAmount, sumToUsdCents } from "@/lib/spend";
import type { Balance, IncomeRow, PaymentMethod } from "@/types/shifts";
import { formatBalance, formatUsd } from "@/lib/format";
import { paymentAccentColor } from "@/lib/paymentColor";
import { useCurrencies } from "@/composables/useCurrencies";
import { grantManagerPass } from "@/composables/useManagerPass";
import { useAuthStore } from "@/stores/auth";
import BalanceReplacementDialog from "@/components/shifts/BalanceReplacementDialog.vue";
import SetOpeningFloatDialog from "@/components/shifts/SetOpeningFloatDialog.vue";

const { currencies, roundingMode } = useCurrencies();
const router = useRouter();
const auth = useAuthStore();

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

const shiftExpenses = ref<any[]>([]);
const shiftLoans = ref<any[]>([]);
async function loadSpend() {
  try {
    const summary = await GetSpendSummary(props.shiftId ?? 0);
    shiftExpenses.value = summary.shiftExpenses ?? [];
    shiftLoans.value = summary.shiftLoans ?? [];
  } catch {
    // The lists just stay as they were; the buttons still work.
  }
}
onMounted(loadSpend);
watch(() => props.shiftId, loadSpend);
// On-account sales are owed, not in the drawer: they stay out of the income
// total and the net, and get their own line under the total instead.
const collectedItems = computed(() => props.items.filter((item) => !item.isOnAccount));
const onAccountTotal = computed(() =>
  props.items.filter((item) => item.isOnAccount).reduce((sum, item) => sum + (Number(item.total) || 0), 0),
);
const collectedTotal = computed(() => (Number(props.incomeTotal) || 0) - onAccountTotal.value);
const netIncome = computed(
  () =>
    collectedTotal.value -
    sumToUsdCents(shiftExpenses.value, currencies.value) -
    sumToUsdCents(shiftLoans.value, currencies.value),
);
const balanceSources = computed<Balance[]>(() => props.balanceSources
  .filter((item) => !item.isOnAccount && !item.isOnCoupon && item.accountId && Number(item.balance) !== 0));

// Admins go straight in; anyone else types the manager password, checked
// by the server, every time they open the page.
const managerDialog = reactive({ show: false, path: "", password: "", error: "", checking: false, reveal: false });
function openManagerPage(path: string) {
  if (auth.isAdmin) {
    void router.push(path);
    return;
  }
  Object.assign(managerDialog, { show: true, path, password: "", error: "", checking: false, reveal: false });
}
async function confirmManager() {
  if (!managerDialog.password || managerDialog.checking) return;
  managerDialog.checking = true;
  managerDialog.error = "";
  try {
    await VerifyManagerPassword(managerDialog.password);
    managerDialog.show = false;
    grantManagerPass(managerDialog.path);
    void router.push(managerDialog.path);
  } catch (error) {
    managerDialog.error = String(error).includes("password") ? "Wrong password" : String(error);
  } finally {
    managerDialog.checking = false;
  }
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

/* A block button is `flex: 1 0 auto`; stop the buttons stretching
   to fill the card's free space so the action group stays compact. */
.shift-income-actions > .k-btn {
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

.spend-button-label {
  display: inline-flex;
  align-items: center;
  font-weight: 700;
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
  background-color: rgba(var(--k-theme-primary), 0.12);
}
</style>
