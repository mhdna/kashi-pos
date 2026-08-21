<template>
  <div class="calculator-shell w-100 h-100 mx-2 px-2 py-2 rounded-xl my-1">
    <CalculatorStatus
      :payment-status="paymentStatus"
      :status-color="statusColor"
      :show-ready-state="showReadyState"
      :remaining-amount="remainingAmount"
    />

    <v-card class="calculator-display pa-2 mb-3" variant="outlined" rounded="lg">
      <span class="calculator-display-value text-h3 font-weight-bold">{{ displayValue }}</span>
    </v-card>

    <CalculatorKeypad @digit="appendDigit" @backspace="backspace" @clear="clearInput" />
    <CalculatorPaymentMethods
      class="mt-3"
      :methods="paymentMethods.filter((method) => !['on coupon', 'on account'].includes(method.name.toLowerCase()))"
      :model-value="activeMethod"
      @select="selectMethodAndConfirm"
    />
    <CalculatorEntries
      :entries="entries"
      :format-amount="formatEntryAmount"
      :currency-symbol-for="currencySymbolFor"
      @remove="removeEntry"
    />
    <v-dialog v-model="couponDialog" max-width="420">
      <v-card title="Coupon payment">
        <v-card-text>
          <v-text-field
            v-model="couponCode"
            :placeholder="selectedCouponType?.referenceOnly ? 'Reference number' : 'Coupon code'"
            :hint="selectedCouponType?.referenceOnly ? 'Enter the reference number for this coupon.' : 'Enter the issued coupon code.'"
            persistent-hint
            autofocus
            variant="outlined"
          />
          <v-select v-model="couponTypeId" :items="couponTypes" item-title="name" item-value="id" placeholder="Coupon type" variant="outlined" />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn @click="couponDialog = false">Cancel</v-btn>
          <v-btn color="primary" @click="confirmCoupon">Add coupon</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import {
  ListCurrencies,
  ListCouponTypes,
  ListPaymentMethods,
  QuotePaymentStatus,
} from "../../wailsjs/go/main/App";
import { formatUsd } from "@/lib/format";
import {
  roundCurrencyUnitsFromCents,
  roundCentsFromCurrencyUnits,
  normalizeRoundingMode,
  type CurrencyRate,
} from "@/lib/money";
import CalculatorEntries from "@/components/calculator/CalculatorEntries.vue";
import CalculatorKeypad from "@/components/calculator/CalculatorKeypad.vue";
import CalculatorPaymentMethods from "@/components/calculator/CalculatorPaymentMethods.vue";
import CalculatorStatus from "@/components/calculator/CalculatorStatus.vue";

interface PaymentMethod {
  id: number;
  name: string;
  displayName: string;
  currencyCode: string;
}
interface Entry {
  type: string;
  amount: number;
  amountCents: number;
  accountId: number;
}

const FALLBACK_CURRENCY: CurrencyRate = { code: "USD", symbol: "$", unitsPerUsd: 1, cashRoundingUnit: 1 };
const props = defineProps<{
  totalCents: number;
  itemsJson?: string;
  roundingMode?: string;
  roundingCurrency?: string;
  clientId?: number;
  hasItems?: boolean;
  exchangeMode?: boolean;
}>();
const emit = defineEmits<{ account: []; coupon: [] }>();
const hasItems = computed(() => Boolean(props.hasItems));
const exchangeMode = computed(() => Boolean(props.exchangeMode));
const roundingMode = computed(() => normalizeRoundingMode(props.roundingMode ?? ""));
const paymentMethods = ref<PaymentMethod[]>([]);
const currencies = ref<(CurrencyRate & { isActive: boolean })[]>([]);
const activeMethod = ref("");
const currentInput = ref("");
const entries = ref<Entry[]>([]);
const couponDialog = ref(false);
const couponCode = ref("");
const couponTypeId = ref<number | null>(null);
const couponTypes = ref<any[]>([]);
const selectedCouponType = computed(() => couponTypes.value.find((item) => item.id === couponTypeId.value));

const roundingCurrency = computed<CurrencyRate>(
  () =>
    currencies.value.find((c) => c.code === props.roundingCurrency) ??
    currencies.value.find((c) => c.cashRoundingUnit > 1) ??
    currencies.value[0] ??
    FALLBACK_CURRENCY,
);

function currencyFor(methodName: string): CurrencyRate {
  const code = paymentMethods.value.find((m) => m.name === methodName)?.currencyCode;
  return currencies.value.find((c) => c.code === code) ?? FALLBACK_CURRENCY;
}

function currencySymbolFor(methodName: string) {
  return currencyFor(methodName).symbol || "$";
}

// totalCents comes straight from the backend quote - no local re-rounding.
const totalCents = computed(() => props.totalCents);
const paidCents = computed(() => entries.value.reduce((sum, entry) => sum + entry.amountCents, 0));

// checkoutReady/cashRoundedToTotal require server-side logic (cash-rounding
// grace, max-overpayment tolerance) and come from ONE backend answer
// (QuotePaymentStatus). diffCents/isPaidInFull use the exact same formula
// backend uses (paid - total) but are recomputed locally and synchronously
// from `entries`, instead of waiting on the debounced backend round trip -
// otherwise a fast sequence of entries (e.g. entering change in two
// currencies back to back) decides each new entry's sign/auto-fill amount
// against a stale diff from before the previous entry was accounted for.
interface BackendPaymentStatus {
  diffCents: number;
  isPaidInFull: boolean;
  isCashRounded: boolean;
  checkoutReady: boolean;
}
const backendStatus = ref<BackendPaymentStatus | null>(null);
let statusRequestToken = 0;

async function refreshPaymentStatus() {
  const token = ++statusRequestToken;
  const itemsJson = props.itemsJson;
  if (!itemsJson || itemsJson === "[]" || entries.value.length === 0) {
    // No payments entered yet is a normal state, not a backend error -
    // QuotePaymentStatus rejects an empty payments array outright.
    backendStatus.value = null;
    return;
  }
  try {
    const result = await QuotePaymentStatus(itemsJson, paymentsJSON.value);
    if (token !== statusRequestToken) return; // a newer request already superseded this one
    backendStatus.value = {
      diffCents: Number(result.diffCents),
      isPaidInFull: Boolean(result.isPaidInFull),
      isCashRounded: Boolean(result.isCashRounded),
      checkoutReady: Boolean(result.checkoutReady),
    };
  } catch (e) {
    if (token !== statusRequestToken) return;
    console.error("QuotePaymentStatus failed:", e);
    backendStatus.value = null;
  }
}

let statusDebounceTimer: ReturnType<typeof setTimeout> | undefined;
watch(
  [entries, () => props.itemsJson],
  () => {
    clearTimeout(statusDebounceTimer);
    statusDebounceTimer = setTimeout(refreshPaymentStatus, 120);
  },
  { deep: true, immediate: true },
);

const diffCents = computed(() => paidCents.value - totalCents.value);
const diff = computed(() => diffCents.value / 100);
const isPaidInFull = computed(() => diffCents.value === 0);
const cashRoundedToTotal = computed(() => backendStatus.value?.isCashRounded ?? false);
const checkoutReady = computed(() => backendStatus.value?.checkoutReady ?? false);
const showReadyState = computed(
  () => checkoutReady.value && (diffCents.value <= 0 || cashRoundedToTotal.value),
);
const paymentStatus = computed(() =>
  isPaidInFull.value
    ? "Paid"
    : cashRoundedToTotal.value
      ? "Paid (cash rounded)"
      : diffCents.value > 0
        ? "Amount to return"
        : "Amount to pay",
);
const statusColor = computed(() =>
  diff.value > 0 ? "text-red-darken-2" : "text-green-darken-2",
);
function formatAmountInCurrency(amountCents: number, currency: CurrencyRate): string {
  if (currency.cashRoundingUnit <= 1) return formatUsd(amountCents / 100);
  return `${currency.symbol} ${roundCurrencyUnitsFromCents(amountCents, currency, roundingMode.value).toLocaleString()}`;
}

const remainingAmount = computed(() => {
  const amountCents = Math.abs(diffCents.value);
  const fullUsd = formatUsd(amountCents / 100);
  const amountsByCurrency = currencies.value
    .filter((c) => c.isActive)
    .map((c) => ({ code: c.code, text: formatAmountInCurrency(amountCents, c) }));
  if (roundingCurrency.value.cashRoundingUnit <= 1) {
    return { usd: fullUsd, lbp: "", fullUsd, fullLbp: "", amountsByCurrency };
  }
  const dollarAmount = amountCents / 100;
  const wholeDollars = Math.floor(dollarAmount / 10) * 10;
  const remainderCents = amountCents - wholeDollars * 100;
  const fullLbp = `${roundingCurrency.value.symbol} ${roundCurrencyUnitsFromCents(amountCents, roundingCurrency.value, roundingMode.value).toLocaleString()}`;
  return {
    usd: `$${wholeDollars.toLocaleString()}`,
    lbp: `${roundingCurrency.value.symbol} ${roundCurrencyUnitsFromCents(remainderCents, roundingCurrency.value, roundingMode.value).toLocaleString()}`,
    fullUsd,
    fullLbp,
    amountsByCurrency,
  };
});
const totalSummary = computed(() => {
  const totalDollars = totalCents.value / 100;
  if (roundingCurrency.value.cashRoundingUnit <= 1) {
    const usd = formatUsd(totalDollars);
    return { usd, lbp: "", split: usd };
  }
  const wholeDollars = Math.floor(totalDollars / 10) * 10;
  const remainderCents = totalCents.value - wholeDollars * 100;
  const currencyLabel = `${roundingCurrency.value.symbol} ${roundCurrencyUnitsFromCents(totalCents.value, roundingCurrency.value, roundingMode.value).toLocaleString()}`;
  return {
    usd: formatUsd(totalDollars),
    lbp: currencyLabel,
    split: `${formatUsd(wholeDollars)} + ${roundingCurrency.value.symbol} ${roundCurrencyUnitsFromCents(remainderCents, roundingCurrency.value, roundingMode.value).toLocaleString()}`,
  };
});
const displayValue = computed(() =>
  currentInput.value ? Number(currentInput.value).toLocaleString() : "0",
);

function formatEntryAmount(amount: number) {
  return amount.toLocaleString(undefined, { maximumFractionDigits: 2 });
}

onMounted(async () => {
  try {
    const methods = await ListPaymentMethods();
    paymentMethods.value = methods.map((method: any) => ({
      id: method.id,
      name: method.name,
      displayName: method.displayName ?? method.name,
      currencyCode: method.currencyCode,
    }));
    activeMethod.value = paymentMethods.value[0]?.name ?? "";
  } catch {
    /* leave payment methods empty */
  }
  try {
    currencies.value = (await ListCurrencies()).map((currency: any) => ({
      code: currency.code,
      symbol: currency.symbol,
      unitsPerUsd: Number(currency.unitsPerUsd),
      cashRoundingUnit: Number(currency.cashRoundingUnit),
      isActive: Boolean(currency.isActive),
    }));
  } catch {
    /* leave currencies empty */
  }
  try { couponTypes.value = (await ListCouponTypes()) as any[]; } catch { couponTypes.value = []; }
});

function appendDigit(value: string) {
  if (value !== "." || !currentInput.value.includes(".")) currentInput.value += value;
}
function backspace() {
  currentInput.value = currentInput.value.slice(0, -1);
}
function clearInput() {
  currentInput.value = "";
}

function confirmEntry(methodName = activeMethod.value) {
  const amount = Number(currentInput.value);
  const method = paymentMethods.value.find((candidate) => candidate.name === methodName);
  const currency = currencyFor(methodName);
  if (!Number.isFinite(amount) || amount <= 0 || currency.unitsPerUsd <= 0) return;
  const amountCents = roundCentsFromCurrencyUnits(amount, currency, roundingMode.value);
  if (amountCents <= 0) return;
  const isChange = diffCents.value > 0;
  entries.value.push({
    type: methodName,
    amount: isChange ? -amount : amount,
    amountCents: isChange ? -amountCents : amountCents,
    accountId: method?.id ?? 0,
  });
  currentInput.value = "";
}

function fillRemainingInput(methodName = activeMethod.value) {
  const settlementCents = Math.abs(diffCents.value);
  const currency = currencyFor(methodName);
  if (settlementCents <= 0 || currency.unitsPerUsd <= 0) return;

  if (currency.cashRoundingUnit > 1) {
    currentInput.value = String(
      roundCurrencyUnitsFromCents(settlementCents, currency, roundingMode.value),
    );
    return;
  }

  const settlementAmount = (settlementCents * currency.unitsPerUsd) / 100;
  currentInput.value = settlementAmount.toFixed(2).replace(/\.00$/, "");
}

function selectMethodAndConfirm(methodName: string) {
  activeMethod.value = methodName;
  if (methodName.toLowerCase() === "on account" && !props.clientId) return;
  if (!currentInput.value) fillRemainingInput(methodName);
  confirmEntry(methodName);
}
function addOnAccount() {
  const method = paymentMethods.value.find((candidate) => candidate.name.toLowerCase() === "on account");
  if (!method || !props.clientId) return;
  const remainingCents = Math.max(-diffCents.value, 0);
  if (remainingCents <= 0) return;
  activeMethod.value = method.name;
  currentInput.value = (remainingCents / 100).toFixed(2);
  confirmEntry();
}
async function openCouponDialog() {
  try { couponTypes.value = (await ListCouponTypes()) as any[]; } catch { couponTypes.value = []; }
  couponCode.value = "";
  couponTypeId.value = null;
  couponDialog.value = true;
}
function confirmCoupon() {
  const type = couponTypes.value.find((item) => item.id === couponTypeId.value);
  if (!couponCode.value.trim() || !type) return;
  addCoupon(couponCode.value, type);
  couponDialog.value = false;
}
function addCoupon(code: string, type: any) {
  const method = paymentMethods.value.find((m) => m.name.toLowerCase() === "on coupon");
  if (!method || !type) return;
  entries.value.push({ type: method.name, amount: Number(type.valueCents) / 100, amountCents: Number(type.valueCents), accountId: method.id });
  (entries.value[entries.value.length - 1] as any).couponCode = code.trim();
  (entries.value[entries.value.length - 1] as any).couponTypeId = type.id;
}
function removeEntry(index: number) {
  entries.value.splice(index, 1);
}
function clearAll() {
  currentInput.value = "";
  entries.value = [];
  activeMethod.value = paymentMethods.value.find((m) => m.currencyCode === "USD")?.name ?? "USD";
}

function holdState() {
  return {
    currentInput: currentInput.value,
    activeMethod: activeMethod.value,
    entries: entries.value.map((entry) => ({ ...entry })),
  };
}

function restoreHoldState(state: any) {
  currentInput.value = String(state?.currentInput ?? "");
  activeMethod.value = String(state?.activeMethod ?? paymentMethods.value[0]?.name ?? "");
  entries.value = Array.isArray(state?.entries)
    ? state.entries.map((entry: any) => ({
        ...entry,
        amount: Number(entry.amount),
        amountCents: Number(entry.amountCents),
        accountId: Number(entry.accountId),
      }))
    : [];
}

const paymentsJSON = computed(() => {
  return JSON.stringify(entries.value.map((entry) => ({
    accountId: entry.accountId,
    amountCents: entry.amountCents,
    entryType: entry.amountCents < 0 ? "change" : "payment",
    ...(entry.type.toLowerCase() === "on coupon" ? { couponCode: (entry as any).couponCode, couponTypeId: (entry as any).couponTypeId } : {}),
  })));
});
const firstEntryAccountId = computed(() => entries.value[0]?.accountId ?? 0);
defineExpose({
  diff,
  checkoutReady,
  totalSummary,
  clearAll,
  firstEntryAccountId,
  paymentsJSON,
  addCoupon,
  selectMethodAndConfirm,
  addOnAccount,
  openCouponDialog,
  holdState,
  restoreHoldState,
});
</script>

<style scoped>
.calculator-display-value {
  font-size: 1.1rem !important;
  line-height: 1.2;
}

</style>

<style scoped>
.calculator-action-button {
  flex: 1 1 0;
  min-width: 0;
}
</style>

<style scoped>
.calculator-display {
  min-height: 42px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-color: rgba(var(--v-theme-on-surface), 0.18);
}
.calculator-special-actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}
</style>
