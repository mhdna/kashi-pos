<template>
  <div class="calculator-shell h-100 mx-2 px-2 py-2 rounded-xl my-1" style="overflow-y: auto">
    <CalculatorStatus
      :payment-status="paymentStatus"
      :status-color="statusColor"
      :show-ready-state="showReadyState"
      :remaining-amount="remainingAmount"
    />

    <v-card class="calculator-display pa-2 mb-3 mt-2" variant="outlined" rounded="lg">
      <v-btn
        class="calculator-display-clear"
        icon
        variant="text"
        size="small"
        aria-label="Clear"
        @click="clearInput"
      >
        <v-icon icon="mdi-close" size="20" />
      </v-btn>
      <span class="calculator-display-value text-h3 font-weight-bold">{{ displayValue }}</span>
      <v-btn
        class="calculator-display-backspace"
        icon
        variant="text"
        size="small"
        aria-label="Backspace"
        @click="backspace"
      >
        <v-icon icon="mdi-backspace-outline" size="20" />
      </v-btn>
    </v-card>

    <div class="d-flex ga-3">
      <div style="flex: 3 3 0; min-width: 0">
        <CalculatorKeypad @digit="appendDigit" />
      </div>
      <div class="calculator-payment-methods-column d-flex flex-column ga-2">
        <CalculatorPaymentMethods
          vertical
          :methods="paymentMethods.filter((method) => !isSpecialPaymentMethod(method.name))"
          :model-value="activeMethod"
          :show-currency="!props.hidePaymentMethodCurrency"
          @select="selectMethodAndConfirm"
        />
        <div class="calculator-action-cards" role="list" aria-label="Other actions">
          <v-card
            class="calculator-type-card d-flex flex-column" style="--payment-accent: #757575"
            :class="{ 'calculator-type-card--disabled': !hasItems || restricted }"
            elevation="0"
            role="button"
            tabindex="0"
            @click="!(!hasItems || restricted) && emit('account')"
            @keydown.enter="!(!hasItems || restricted) && emit('account')"
            @keydown.space.prevent="!(!hasItems || restricted) && emit('account')"
          >
            <v-card-text class="text-center py-2 px-1 flex-grow-1 d-flex align-center justify-center">
              <span class="payment-method-name text-h6 font-weight-bold">On Account</span>
            </v-card-text>
          </v-card>
          <v-card
            class="calculator-type-card d-flex flex-column" style="--payment-accent: #757575"
            :class="{ 'calculator-type-card--disabled': restricted }"
            elevation="0"
            role="button"
            tabindex="0"
            @click="!restricted && emit('coupon')"
            @keydown.enter="!restricted && emit('coupon')"
            @keydown.space.prevent="!restricted && emit('coupon')"
          >
            <v-card-text class="text-center py-2 px-1 flex-grow-1 d-flex align-center justify-center">
              <span class="payment-method-name text-h6 font-weight-bold">Coupon</span>
            </v-card-text>
          </v-card>
        </div>
      </div>
    </div>

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

    <v-dialog :model-value="Boolean(pendingOverchange)" max-width="420" @update:model-value="(v) => !v && cancelOverchange()">
      <v-card title="Confirm change amount" v-if="pendingOverchange">
        <v-card-text>
          Give {{ pendingOverchange.currencySymbol }} {{ pendingOverchange.amount.toLocaleString() }} in change?
          This is more than what's owed.
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn @click="cancelOverchange">Cancel</v-btn>
          <v-btn color="primary" @click="confirmOverchange">Confirm</v-btn>
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
  QuoteSettlementPaymentStatus,
  QuoteExchangePaymentStatus,
} from "../../wailsjs/go/main/App";
import { formatUsd } from "@/lib/format";
import {
  roundCurrencyUnitsFromCents,
  roundCentsFromCurrencyUnits,
  normalizeRoundingMode,
  isRoundedToUnit,
  type CurrencyRate,
} from "@/lib/money";
import CalculatorEntries from "@/components/calculator/CalculatorEntries.vue";
import CalculatorKeypad from "@/components/calculator/CalculatorKeypad.vue";
import CalculatorPaymentMethods from "@/components/calculator/CalculatorPaymentMethods.vue";
import CalculatorStatus from "@/components/calculator/CalculatorStatus.vue";
import { isSpecialPaymentMethod } from "@/lib/paymentMethods";

interface PaymentMethod {
  id: number;
  name: string;
  displayName: string;
  currencyCode: string;
  color: string;
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
  // When set, this is "change settlement" mode: payment sufficiency is
  // validated against this existing invoice's frozen GrandTotal
  // (QuoteSettlementPaymentStatus) instead of re-pricing itemsJson.
  settlementInvoiceId?: number;
  // When set, this is exchange mode: itemsJson contains negative-qty
  // returned lines that QuotePaymentStatus's normal item parsing rejects
  // outright, so payment sufficiency is validated via
  // QuoteExchangePaymentStatus against this original sale invoice instead.
  exchangeOriginalInvoiceId?: number;
  hidePaymentMethodCurrency?: boolean;
  // When enabled, confirming an entry against an LBP-like (cash-rounded)
  // payment method multiplies the typed amount by 1,000 - so a cashier can
  // type "50" instead of "50000" or pressing the "00" key.
  autoAppendLbpZeros?: boolean;
}>();
const emit = defineEmits<{ account: []; coupon: [] }>();
const hasItems = computed(() => Boolean(props.hasItems));
const exchangeMode = computed(() => Boolean(props.exchangeMode));
// Both exchange and settlement mode lock the client and disable
// on-account/coupon - only the calculator and the mode-specific checkout
// action are active.
const restricted = computed(() => exchangeMode.value || Boolean(props.settlementInvoiceId));
const roundingMode = computed(() => normalizeRoundingMode(props.roundingMode ?? ""));
const paymentMethods = ref<PaymentMethod[]>([]);
const currencies = ref<(CurrencyRate & { isActive: boolean; isDefault: boolean })[]>([]);
const activeMethod = ref("");
const currentInput = ref("");
const entries = ref<Entry[]>([]);
const couponDialog = ref(false);
const couponCode = ref("");
const couponTypeId = ref<number | null>(null);
const couponTypes = ref<any[]>([]);
const selectedCouponType = computed(() => couponTypes.value.find((item) => item.id === couponTypeId.value));
const pendingOverchange = ref<{ methodName: string; amount: number; amountCents: number; currencySymbol: string } | null>(null);

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
  // With zero entries there's nothing to validate against a normal sale's
  // itemsJson (still gated below), but settlement/exchange mode must still
  // be asked - an exchange that already nets to $0 (or a settlement already
  // balanced) is checkout-ready with no entries at all, and the backend
  // quote endpoints accept an empty payments array (paid = 0) precisely for
  // this case.
  if (entries.value.length === 0 && !props.settlementInvoiceId && !props.exchangeOriginalInvoiceId) {
    backendStatus.value = null;
    return;
  }
  try {
    const result = props.settlementInvoiceId
      ? await QuoteSettlementPaymentStatus(props.settlementInvoiceId, paymentsJSON.value)
      : props.exchangeOriginalInvoiceId
        ? await QuoteExchangePaymentStatus(
            props.itemsJson ?? "[]",
            paymentsJSON.value,
            props.exchangeOriginalInvoiceId,
          )
        : props.itemsJson && props.itemsJson !== "[]"
          ? await QuotePaymentStatus(props.itemsJson, paymentsJSON.value)
          : null;
    if (token !== statusRequestToken) return; // a newer request already superseded this one
    backendStatus.value = result && {
      diffCents: Number(result.diffCents),
      isPaidInFull: Boolean(result.isPaidInFull),
      isCashRounded: Boolean(result.isCashRounded),
      checkoutReady: Boolean(result.checkoutReady),
    };
  } catch (e) {
    if (token !== statusRequestToken) return;
    console.error("Payment status quote failed:", e);
    backendStatus.value = null;
  }
}

let statusDebounceTimer: ReturnType<typeof setTimeout> | undefined;
watch(
  [entries, () => props.itemsJson, () => props.settlementInvoiceId, () => props.exchangeOriginalInvoiceId],
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
        ? "Give"
        : "Get",
);
const statusColor = computed(() =>
  diff.value > 0 ? "text-red-darken-2" : "calculator-green",
);
function formatAmountInCurrency(amountCents: number, currency: CurrencyRate): string {
  if (currency.cashRoundingUnit <= 1) return formatUsd(amountCents / 100);
  return `${currency.symbol} ${roundCurrencyUnitsFromCents(amountCents, currency, roundingMode.value).toLocaleString()}`;
}

const remainingAmount = computed(() => {
  const amountCents = Math.abs(diffCents.value);
  const fullUsd = formatUsd(amountCents / 100);
  const amountsByCurrency = currencies.value
    // The default currency (e.g. USC, the internal accounting cent) is a
    // bookkeeping unit, not something a cashier should ever see as a total.
    .filter((c) => c.isActive && !c.isDefault)
    .map((c) => ({
      code: c.code,
      text: formatAmountInCurrency(amountCents, c),
      rounded: isRoundedToUnit(amountCents, c, roundingMode.value),
    }));
  if (roundingCurrency.value.cashRoundingUnit <= 1) {
    return { usd: fullUsd, lbp: "", fullUsd, fullLbp: "", amountsByCurrency, lbpRounded: false };
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
    lbpRounded: isRoundedToUnit(remainderCents, roundingCurrency.value, roundingMode.value),
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
      color: method.color ?? "",
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
      isDefault: Boolean(currency.isDefault),
    }));
  } catch {
    /* leave currencies empty */
  }
  try { couponTypes.value = (await ListCouponTypes()) as any[]; } catch { couponTypes.value = []; }
});

// Tracks whether currentInput was populated by fillRemainingInput (an
// already-final amount) rather than typed digit-by-digit - autoAppendLbpZeros
// only multiplies manually typed amounts, never an auto-filled one.
const inputWasAutoFilled = ref(false);

function appendDigit(value: string) {
  inputWasAutoFilled.value = false;
  if (value !== "." || !currentInput.value.includes(".")) currentInput.value += value;
}
function backspace() {
  inputWasAutoFilled.value = false;
  currentInput.value = currentInput.value.slice(0, -1);
}
function clearInput() {
  inputWasAutoFilled.value = false;
  currentInput.value = "";
}

function pushEntry(methodName: string, amount: number, amountCents: number, isChange: boolean) {
  const method = paymentMethods.value.find((candidate) => candidate.name === methodName);
  entries.value.push({
    type: methodName,
    amount: isChange ? -amount : amount,
    amountCents: isChange ? -amountCents : amountCents,
    accountId: method?.id ?? 0,
  });
}

function confirmEntry(methodName = activeMethod.value) {
  const currency = currencyFor(methodName);
  const shouldAppendZeros =
    props.autoAppendLbpZeros &&
    !inputWasAutoFilled.value &&
    currency.cashRoundingUnit > 1 &&
    !currentInput.value.includes(".");
  const amount = Number(currentInput.value) * (shouldAppendZeros ? 1000 : 1);
  if (!Number.isFinite(amount) || amount <= 0 || currency.unitsPerUsd <= 0) return;
  const amountCents = roundCentsFromCurrencyUnits(amount, currency, roundingMode.value);
  if (amountCents <= 0) return;
  const isChange = diffCents.value > 0;
  // Change larger than what's owed is a legitimate move (e.g. breaking a
  // 100,000 LL note when only 50,000 is owed, with the customer handing
  // back the 50,000 difference in smaller notes) - so it must never be
  // blocked. But it's also exactly what a mistyped amount looks like, so
  // ask for confirmation instead of silently accepting or silently
  // rejecting it.
  if (isChange && amountCents > diffCents.value) {
    pendingOverchange.value = { methodName, amount, amountCents, currencySymbol: currency.symbol };
    return;
  }
  pushEntry(methodName, amount, amountCents, isChange);
  currentInput.value = "";
  inputWasAutoFilled.value = false;
}

function confirmOverchange() {
  if (!pendingOverchange.value) return;
  const { methodName, amount, amountCents } = pendingOverchange.value;
  pushEntry(methodName, amount, amountCents, true);
  pendingOverchange.value = null;
  currentInput.value = "";
  inputWasAutoFilled.value = false;
}

function cancelOverchange() {
  pendingOverchange.value = null;
}

function fillRemainingInput(methodName = activeMethod.value) {
  const settlementCents = Math.abs(diffCents.value);
  const currency = currencyFor(methodName);
  if (settlementCents <= 0 || currency.unitsPerUsd <= 0) return;

  if (currency.cashRoundingUnit > 1) {
    currentInput.value = String(
      roundCurrencyUnitsFromCents(settlementCents, currency, roundingMode.value),
    );
    inputWasAutoFilled.value = true;
    return;
  }

  const settlementAmount = (settlementCents * currency.unitsPerUsd) / 100;
  currentInput.value = settlementAmount.toFixed(2).replace(/\.00$/, "");
  inputWasAutoFilled.value = true;
}

function selectMethodAndConfirm(methodName: string) {
  activeMethod.value = methodName;
  if (methodName.toLowerCase() === "on account" && !props.clientId) return;
  if (!currentInput.value) fillRemainingInput(methodName);
  confirmEntry(methodName);
  // Each card click adds one payment entry immediately (there's no separate
  // "confirm" step) - clearing the selection right after keeps the card from
  // looking permanently highlighted once its payment has already been added.
  activeMethod.value = "";
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
  font-size: 1.6rem !important;
  line-height: 1.2;
  flex: 1 1 0;
  text-align: center;
  color: rgba(var(--v-theme-on-surface), 0.7);
}
</style>

<style scoped>
.calculator-payment-methods-column {
  flex: 2 2 0;
  min-width: 0;
}

.calculator-action-cards {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}
.calculator-type-card {
  cursor: pointer;
  background: transparent !important;
  box-shadow: none !important;
  border: 1px solid rgba(var(--v-border-color), 0.18);
  border-top: 10px solid var(--payment-accent);
  transition: border-color 120ms ease;
}
.payment-method-name {
  font-size: 1.1rem !important;
  line-height: 1.25;
  color: rgb(var(--v-theme-on-surface));
}
.calculator-type-card--disabled {
  opacity: 0.5;
  pointer-events: none;
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
