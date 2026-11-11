<template>
  <k-sheet class="shift-income-card shifts-card pa-4" style="flex: 1; min-width: 0">
    <div class="shift-income-header" data-test="income-header">
      <span class="shift-income-header-icon"><k-icon icon="mdi-cash-register" size="22" /></span>
      <span class="shift-income-header-title">{{ title || "Shift Income" }}</span>
      <span v-if="shiftId" class="shift-income-header-shift" data-test="income-shift">Shift #{{ shiftId }}</span>
    </div>
    <!-- Everything between the title and the buttons scrolls on its own, so
         the buttons stay at the bottom of the card however long it gets. -->
    <div class="shift-income-body">
    <k-progress-linear v-if="loading" indeterminate />
    <template v-else-if="items.length">
      <k-list density="compact" class="pa-0">
        <k-list-item v-for="item in collectedItems" :key="item.accountName" class="shift-income-row">
        <template #prepend>
          <k-avatar :color="paymentAccentColor({ name: item.accountName, color: item.color })" variant="tonal" size="32">
            <k-icon :icon="paymentIcon({ name: item.accountName, isOnAccount: item.isOnAccount })" size="18" data-test="account-icon" />
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
      <div class="shift-total" data-test="income-total">
        <span class="shift-total-label">Total collected</span>
        <span class="shift-total-amount">{{ formatBalance(collectedTotal, "USD", currencies, roundingMode) }}</span>
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
    <div v-if="shiftId && (onAccountTotal || shiftExpenses.length || shiftLoans.length)" class="shift-income-extras">
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
      <div v-if="shiftExpenses.length || shiftLoans.length" class="shift-net" data-test="net-income">
        <span>Net income</span>
        <span class="shift-net-amount">{{ formatBalance(netIncome, "USD", currencies, roundingMode) }}</span>
      </div>
    </div>
    </div>
    <div v-if="shiftId" class="shift-income-actions mt-auto">
      <k-btn
        v-if="balanceSources.length > 0"
        block
        class="shift-income-secondary"
        prepend-icon="mdi-swap-horizontal"
        variant="outlined"
        @click="replacementOpen = true"
      >
        Swap balance
      </k-btn>
      <k-btn
        v-if="openingFloatEnabled"
        block
        class="shift-income-secondary"
        prepend-icon="mdi-cash-plus"
        variant="outlined"
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
import { collectedTotalCents, netIncomeCents, onAccountTotalCents } from "@/lib/shiftIncome";
import type { Balance, IncomeRow, PaymentMethod } from "@/types/shifts";
import { formatBalance, formatUsd } from "@/lib/format";
import { paymentAccentColor } from "@/lib/paymentColor";
import { paymentIcon } from "@/lib/paymentIcon";
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
const onAccountTotal = computed(() => onAccountTotalCents(props.items));
const collectedTotal = computed(() => collectedTotalCents(props.items));
const netIncome = computed(() =>
  netIncomeCents(
    collectedTotal.value,
    sumToUsdCents(shiftExpenses.value, currencies.value) + sumToUsdCents(shiftLoans.value, currencies.value),
  ),
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

/* Spans the card edge to edge (cancelling the card's 16px padding, and putting
   it back inside) so the total band can fill the width: a scrolling box clips
   anything that bleeds past it. */
.shift-income-body {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  margin-inline: -16px;
  padding-inline: 16px;
}

/* Centred: the icon in a soft disc, the title under it, the shift number as a
   small pill under that; a hairline under the lot separates it from the lines. */
.shift-income-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 4px 0 14px;
  margin-bottom: 4px;
  text-align: center;
  border-bottom: 1px solid rgba(var(--k-border-color), var(--k-border-opacity));
}
.shift-income-header-icon {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border-radius: 50%;
  background: rgba(67, 160, 71, 0.14);
  color: #2e7d32;
}
.shift-income-header-title {
  min-width: 0;
  font-size: 1.05rem;
  font-weight: 700;
  line-height: 1.2;
}
.shift-income-header-shift {
  flex: 0 0 auto;
  padding: 3px 10px;
  border-radius: 999px;
  background: rgba(var(--k-theme-on-surface), 0.08);
  font-size: 0.78rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  white-space: nowrap;
}

.shift-income-empty {
  display: flex;
  align-items: center;
  flex-direction: column;
}

.shift-income-empty-message {
  width: 100%;
}

/* One footer: a rule above it, then the actions at one height. */
.shift-income-actions {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  gap: 8px;
  margin-top: 16px;
  padding-top: 14px;
  border-top: 1px solid rgba(var(--k-border-color), var(--k-border-opacity));
}
.shift-income-secondary {
  min-height: 44px;
}
/* Rows start at the card's own left edge, like the title above them. */
.shift-income-row {
  padding-inline: 0 !important;
  min-height: 52px;
}
.shift-income-row + .shift-income-row {
  border-top: 1px solid rgba(var(--k-border-color), 0.5);
}
.shift-income-extras {
  padding-top: 8px;
}

/* A block button is `flex: 1 0 auto`; stop the buttons stretching
   to fill the card's free space so the action group stays compact. */
.shift-income-actions > .k-btn {
  flex: 0 0 auto;
}

/* The total is the card's answer: a square band across the whole card (it
   cancels the card's 16px padding), a rule above it, a light green tint behind,
   bigger than the lines it adds up, amount in figures that line up. */
.shift-total {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin: 12px -16px 4px;
  padding: 14px 16px;
  border-top: 1px solid rgba(var(--k-border-color), var(--k-border-opacity));
  border-radius: 0;
  background: rgba(67, 160, 71, 0.13);
}
.shift-total-label {
  font-size: 0.95rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  opacity: 0.8;
}
.shift-total-amount {
  font-size: 1.3rem;
  font-weight: 800;
  line-height: 1.1;
  font-variant-numeric: tabular-nums;
}
.shift-net {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  padding: 8px 12px 0;
  font-weight: 600;
}
.shift-net-amount {
  font-weight: 800;
  font-variant-numeric: tabular-nums;
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
  min-height: 48px;
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
