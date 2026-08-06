<template>
  <div class="calculator-shell w-100 h-100 mx-2 px-2 py-2 rounded-xl my-1">
    <CalculatorStatus
      :payment-status="paymentStatus"
      :status-color="statusColor"
      :show-ready-state="showReadyState"
      :remaining-amount="remainingAmount"
      :total-summary="totalSummary"
    />

    <v-card class="calculator-display pa-2 mb-3" variant="outlined" rounded="lg">
      <span class="text-h5 font-weight-bold">{{ displayValue }}</span>
    </v-card>

    <CalculatorKeypad @digit="appendDigit" @backspace="backspace" @clear="clearInput" />
    <CalculatorPaymentMethods
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
            :label="selectedCouponType?.referenceOnly ? 'Reference number' : 'Coupon code'"
            :hint="selectedCouponType?.referenceOnly ? 'Enter the reference number for this coupon.' : 'Enter the issued coupon code.'"
            persistent-hint
            autofocus
            variant="outlined"
          />
          <v-select v-model="couponTypeId" :items="couponTypes" item-title="name" item-value="id" label="Coupon type" variant="outlined" />
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
import { computed, onMounted, ref } from "vue";
import { GetAppSettings, ListCurrencies, ListCouponTypes, ListPaymentMethods } from "../../wailsjs/go/main/App";
import { formatUsd } from "@/lib/format";
import {
  isCashRoundedEquivalent,
  normalizeCashRoundedPayments,
  roundLbpFromCents,
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
interface Currency {
  code: string;
  valueInDefaultCurrency: number;
}
interface Entry {
  type: string;
  amount: number;
  amountCents: number;
  accountId: number;
}

const DEFAULT_EXCHANGE_RATE = 90000;
const DEFAULT_USD_RATE = 100;
const props = defineProps<{ total: number; roundingMode?: string; clientId?: number }>();
const roundingMode = computed(() => props.roundingMode ?? "half_up");
const paymentMethods = ref<PaymentMethod[]>([]);
const currencies = ref<Currency[]>([]);
const exchangeRate = ref(DEFAULT_EXCHANGE_RATE);
const maxOverpaymentCents = ref(0);
const activeMethod = ref("");
const currentInput = ref("");
const entries = ref<Entry[]>([]);
const couponDialog = ref(false);
const couponCode = ref("");
const couponTypeId = ref<number | null>(null);
const couponTypes = ref<any[]>([]);
const selectedCouponType = computed(() => couponTypes.value.find((item) => item.id === couponTypeId.value));

const usdRate = computed(
  () => currencies.value.find((c) => c.code === "USD")?.valueInDefaultCurrency ?? DEFAULT_USD_RATE,
);

function currencySymbolFor(methodName: string) {
  return paymentMethods.value.find((m) => m.name === methodName)?.currencyCode === "LBP"
    ? "LBP"
    : "$";
}

const totalCents = computed(() => Math.round(props.total * 100));
const paidCents = computed(() => entries.value.reduce((sum, entry) => sum + entry.amountCents, 0));
const diffCents = computed(() => paidCents.value - totalCents.value);
const overpaymentAllowed = computed(() => diffCents.value <= maxOverpaymentCents.value);
const diff = computed(() => diffCents.value / 100);
const isPaidInFull = computed(() => diffCents.value === 0);
const roundedShortfallLbp = computed(() =>
  diffCents.value < 0
    ? roundLbpFromCents(-diffCents.value, exchangeRate.value, roundingMode.value)
    : 0,
);
const cashRoundedToTotal = computed(() =>
  isCashRoundedEquivalent(
    paidCents.value,
    totalCents.value,
    exchangeRate.value,
    roundingMode.value,
  ),
);
const checkoutReady = computed(
  () =>
    (diffCents.value >= 0 && (overpaymentAllowed.value || cashRoundedToTotal.value)) ||
    (diffCents.value < 0 && roundedShortfallLbp.value === 0),
);
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
const remainingAmount = computed(() => {
  const amountCents = Math.abs(diffCents.value);
  const dollarAmount = amountCents / 100;
  const wholeDollars = Math.floor(dollarAmount / 10) * 10;
  const remainderCents = amountCents - wholeDollars * 100;
  return {
    usd: `$${wholeDollars.toLocaleString()}`,
    lbp: `L.L. ${roundLbpFromCents(remainderCents, exchangeRate.value, roundingMode.value).toLocaleString()}`,
  };
});
const totalSummary = computed(() => {
  const wholeDollars = Math.floor(props.total / 10) * 10;
  const remainderCents = totalCents.value - wholeDollars * 100;
  return {
    usd: formatUsd(props.total),
    lbp: `L.L. ${roundLbpFromCents(totalCents.value, exchangeRate.value, roundingMode.value).toLocaleString()}`,
    split: `${formatUsd(wholeDollars)} + L.L. ${roundLbpFromCents(remainderCents, exchangeRate.value, roundingMode.value).toLocaleString()}`,
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
    const settings = await GetAppSettings();
    const configuredRate = Number(settings?.exchangeRate);
    if (Number.isFinite(configuredRate) && configuredRate > 0) exchangeRate.value = configuredRate;
    const configuredMaxOverpayment = Number(settings?.maxOverpaymentCents);
    if (Number.isFinite(configuredMaxOverpayment) && configuredMaxOverpayment >= 0) {
      maxOverpaymentCents.value = Math.round(configuredMaxOverpayment);
    }
  } catch {
    /* keep default exchange rate */
  }
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
      valueInDefaultCurrency: Number(currency.valueInDefaultCurrency),
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

function paymentRate(methodName: string) {
  const method = paymentMethods.value.find((candidate) => candidate.name === methodName);
  const currencyCode = method?.currencyCode?.toUpperCase() ?? "";
  return currencyCode === "LBP"
    ? exchangeRate.value * usdRate.value
    : (currencies.value.find((currency) => currency.code === currencyCode)
        ?.valueInDefaultCurrency ?? 0);
}

function confirmEntry(methodName = activeMethod.value) {
  const amount = Number(currentInput.value);
  const method = paymentMethods.value.find((candidate) => candidate.name === methodName);
  const methodRate = paymentRate(methodName);
  if (!Number.isFinite(amount) || amount <= 0 || methodRate <= 0 || usdRate.value <= 0)
    return;
  const amountCents = Math.round((amount * usdRate.value * 100) / methodRate);
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
  const settlementCents = Math.abs(totalCents.value - paidCents.value);
  const method = paymentMethods.value.find((candidate) => candidate.name === methodName);
  const currencyCode = method?.currencyCode?.toUpperCase() ?? "";
  const methodRate = paymentRate(methodName);
  if (settlementCents <= 0 || methodRate <= 0 || usdRate.value <= 0) return;

  if (currencyCode === "LBP") {
    currentInput.value = String(
      roundLbpFromCents(settlementCents, exchangeRate.value, roundingMode.value),
    );
    return;
  }

  const settlementAmount = (settlementCents * methodRate) / (usdRate.value * 100);
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
  const remainingCents = Math.max(totalCents.value - paidCents.value, 0);
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

const paymentsJSON = computed(() => {
  const payments = entries.value.map((entry) => ({
    accountId: entry.accountId,
    amountCents: entry.amountCents,
    entryType: entry.amountCents < 0 ? "change" : "payment",
    ...(entry.type.toLowerCase() === "on coupon" ? { couponCode: (entry as any).couponCode, couponTypeId: (entry as any).couponTypeId } : {}),
  }));
  if (checkoutReady.value) {
    const normalizedAmounts = normalizeCashRoundedPayments(
      payments.map((payment) => payment.amountCents),
      totalCents.value,
      exchangeRate.value,
      roundingMode.value,
      entries.value.map(
        (entry) => !["on coupon", "on account"].includes(entry.type.toLowerCase()),
      ),
    );
    payments.forEach((payment, index) => {
      payment.amountCents = normalizedAmounts[index]!;
    });
  }
  return JSON.stringify(payments);
});
const firstEntryAccountId = computed(() => entries.value[0]?.accountId ?? 0);
defineExpose({
  diff,
  checkoutReady,
  clearAll,
  firstEntryAccountId,
  paymentsJSON,
  addCoupon,
  selectMethodAndConfirm,
  addOnAccount,
  openCouponDialog,
});
</script>

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
