<template>
  <div class="calculator-shell h-100 mx-2 px-2 py-2 rounded-xl my-1" style="overflow-y: auto">
    <CalculatorStatus
      :payment-status="paymentStatus"
      :status-color="statusColor"
      :show-ready-state="showReadyState"
      :remaining-amount="remainingAmount"
      :quote-error="quoteError"
      :exchange-shortfall-cents="exchangeShortfallCents"
      :rounded-from-text="roundedFromText"
      :invoice-discount-text="invoiceDiscountText"
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
            v-if="selectedCouponType?.codeRequired !== false"
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
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import {
  ListCurrencies,
  ListCouponTypes,
  ListPaymentMethods,
  QuotePaymentStatus,
  QuoteSettlementPaymentStatus,
  QuoteExchangePaymentStatus,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
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
import { isCashPaymentMethod, isSpecialPaymentMethod } from "@/lib/paymentMethods";

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
  // The total before the backend's cash rounding; see roundedFromText.
  unroundedCents?: number;
  // kashi's invoice discount on this sale (invoice_discounts.go), already
  // off totalCents; 0 for none.
  invoiceDiscountCents?: number;
  invoiceDiscountName?: string;
  // Set when the backend quote failed. totalCents is meaningless (0) in that
  // case, so the total is replaced by this message and checkout is blocked -
  // otherwise diffCents === 0 makes a pricing failure read as "Paid".
  quoteError?: string | null;
  // Exchange mode only: how much more replacement value is still needed to
  // cover the returned items. Exchanges never pay cash back to the
  // customer, so while this is > 0 checkoutReady stays false and the status
  // bar shows this instead of a paid/remaining amount.
  exchangeShortfallCents?: number;
  itemsJson?: string;
  roundingMode?: string;
  roundingCurrency?: string;
  // The first half of the split total. Empty falls back to FALLBACK_CURRENCY
  // (USD), which is what this half was hardcoded to before it was per-branch
  // config. The second half is roundingCurrency.
  splitPrimaryCurrency?: string;
  // False shows the rounding currency first. Defaults to true.
  splitPrimaryFirst?: boolean;
  // The split's own denomination, independent of the primary currency's
  // cash_rounding_unit -- that field also drives the headline total's own
  // rounding when that currency is picked on its own (formatAmountInCurrency
  // below), which is a different concern. Unset/0 falls back to
  // cash_rounding_unit, matching the split's original behaviour.
  splitPrimaryStep?: number;
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
  // Defaults to true. When false, entering change/refund larger than what's
  // owed back is accepted straight away instead of prompting "Confirm change
  // amount".
  overchangeConfirmationEnabled?: boolean;
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

// The total is rounded so it can be paid in cash (e.g. $5.60 becomes
// $5 + LL 50,000 = $5.56); this says from what, so the cashier can see why it
// differs from the lines.
const roundedFromText = computed(() => {
  const unrounded = props.unroundedCents;
  if (unrounded == null || unrounded === totalCents.value) return "";
  return `Rounded from $${String(Number((unrounded / 100).toFixed(2)))}`;
});
const invoiceDiscountText = computed(() => {
  const cents = props.invoiceDiscountCents ?? 0;
  if (cents <= 0) return "";
  return `${props.invoiceDiscountName || "Invoice discount"} −$${String(Number((cents / 100).toFixed(2)))}`;
});
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
    // No wait, for the same reason as home.vue's quote: the backend answers
    // in well under a millisecond; statusRequestToken drops stale replies.
    clearTimeout(statusDebounceTimer);
    statusDebounceTimer = setTimeout(refreshPaymentStatus, 0);
  },
  { deep: true, immediate: true },
);

const quoteFailed = computed(() => Boolean(props.quoteError));
const exchangeShortfallCents = computed(() => props.exchangeShortfallCents ?? 0);
const diffCents = computed(() => paidCents.value - totalCents.value);
const diff = computed(() => diffCents.value / 100);
const isPaidInFull = computed(() => !quoteFailed.value && diffCents.value === 0);
const cashRoundedToTotal = computed(() => backendStatus.value?.isCashRounded ?? false);
const checkoutReady = computed(
  () =>
    !quoteFailed.value &&
    exchangeShortfallCents.value <= 0 &&
    (backendStatus.value?.checkoutReady ?? false),
);
const showReadyState = computed(
  () => checkoutReady.value && (diffCents.value <= 0 || cashRoundedToTotal.value),
);
const paymentStatus = computed(() =>
  quoteFailed.value
    ? "Pricing failed"
    : isPaidInFull.value
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

// The first half of the split. Falls back to USD rather than to the default
// currency, which would be the accounting cent -- never a total a cashier
// should read.
const splitPrimary = computed<CurrencyRate>(() => {
  const found = currencies.value.find((c) => c.code === props.splitPrimaryCurrency);
  return found && found.unitsPerUsd > 0 ? found : FALLBACK_CURRENCY;
});

// A one-character symbol sits flush against its number ("$40"); a word-like
// one gets a space ("LL 439,000"). Matches how each was written when the two
// halves were hardcoded.
function unitsLabel(currency: CurrencyRate, units: number): string {
  const sep = currency.symbol.length === 1 ? "" : " ";
  return `${currency.symbol}${sep}${units.toLocaleString()}`;
}

// Splits an amount into whole units of the primary currency plus whatever is
// left over, expressed in the secondary.
//
// The primary half truncates to that currency's own cash_rounding_unit -- the
// largest denomination its cash actually rounds to. That replaces a hardcoded
// 10 ("whole tens of dollars"), which was this same idea with USD assumed.
function splitAmount(amountCents: number) {
  const primary = splitPrimary.value;
  const secondary = roundingCurrency.value;
  const step = Math.max(1, props.splitPrimaryStep || primary.cashRoundingUnit);

  const primaryUnits = (amountCents / 100) * primary.unitsPerUsd;
  const wholePrimary = Math.floor(primaryUnits / step) * step;
  const remainderCents = amountCents - (wholePrimary / primary.unitsPerUsd) * 100;

  const primaryText = unitsLabel(primary, wholePrimary);
  const secondaryText = unitsLabel(
    secondary,
    roundCurrencyUnitsFromCents(remainderCents, secondary, roundingMode.value),
  );
  // The order is display-only: the arithmetic above is always primary-first.
  const primaryFirst = props.splitPrimaryFirst !== false;
  return {
    first: primaryFirst ? primaryText : secondaryText,
    second: primaryFirst ? secondaryText : primaryText,
    remainderCents,
  };
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
  // A secondary currency that rounds to 1 has no denomination to split at, so
  // there is no meaningful "whole units + remainder" to show -- one line only.
  if (roundingCurrency.value.cashRoundingUnit <= 1) {
    return { first: fullUsd, second: "", fullPrimary: fullUsd, fullSecondary: "", amountsByCurrency, secondaryRounded: false };
  }
  const { first, second, remainderCents } = splitAmount(amountCents);
  return {
    first,
    second,
    fullPrimary: fullUsd,
    fullSecondary: unitsLabel(
      roundingCurrency.value,
      roundCurrencyUnitsFromCents(amountCents, roundingCurrency.value, roundingMode.value),
    ),
    amountsByCurrency,
    secondaryRounded: isRoundedToUnit(remainderCents, roundingCurrency.value, roundingMode.value),
  };
});
const totalSummary = computed(() => {
  const totalDollars = totalCents.value / 100;
  if (roundingCurrency.value.cashRoundingUnit <= 1) {
    const usd = formatUsd(totalDollars);
    return { first: usd, second: "", split: usd };
  }
  const { first, second } = splitAmount(totalCents.value);
  return {
    first: formatUsd(totalDollars),
    second: unitsLabel(
      roundingCurrency.value,
      roundCurrencyUnitsFromCents(totalCents.value, roundingCurrency.value, roundingMode.value),
    ),
    split: `${first} + ${second}`,
  };
});
const displayValue = computed(() =>
  currentInput.value ? Number(currentInput.value).toLocaleString() : "0",
);

function formatEntryAmount(amount: number) {
  return amount.toLocaleString(undefined, { maximumFractionDigits: 2 });
}

async function loadPaymentMethods() {
  try {
    const methods = await ListPaymentMethods();
    paymentMethods.value = methods.map((method: any) => ({
      id: method.id,
      name: method.name,
      displayName: method.displayName ?? method.name,
      currencyCode: method.currencyCode,
      color: method.color ?? "",
    }));
    if (!paymentMethods.value.some((m) => m.name === activeMethod.value)) {
      activeMethod.value = paymentMethods.value[0]?.name ?? "";
    }
  } catch {
    /* leave payment methods as-is */
  }
}

async function loadCurrencies() {
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
    /* leave currencies as-is */
  }
}

// Backend sync (poll or WS push) applies payment-method/currency changes
// straight to the local DB with no way for an already-open Calculator to
// know its data went stale - reload on the same sync_applied:<entity>
// signal shifts.vue uses. See sync_pull.go.
let stopSyncListeners: (() => void)[] = [];

onMounted(async () => {
  stopSyncListeners = [
    EventsOn("sync_applied:cashbox_accounts", loadPaymentMethods),
    EventsOn("sync_applied:currencies", loadCurrencies),
  ];
  await loadPaymentMethods();
  await loadCurrencies();
  try { couponTypes.value = (await ListCouponTypes()) as any[]; } catch { couponTypes.value = []; }
});

onBeforeUnmount(() => {
  stopSyncListeners.forEach((stop) => stop());
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
  if (isChange && amountCents > diffCents.value && props.overchangeConfirmationEnabled !== false) {
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
  if (!type || (type.codeRequired !== false && !couponCode.value.trim())) return;
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
// Total physical cash tendered this sale (gross, not netted against change
// given back) - shown to the cashier while the receipt prints.
const cashTotalCents = computed(() =>
  entries.value
    .filter((entry) => entry.amountCents > 0 && isCashPaymentMethod(entry.type))
    .reduce((sum, entry) => sum + entry.amountCents, 0),
);
defineExpose({
  diff,
  checkoutReady,
  totalSummary,
  clearAll,
  firstEntryAccountId,
  paymentsJSON,
  cashTotalCents,
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
