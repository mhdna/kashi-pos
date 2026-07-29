<template>
  <v-card flat class="calculator-shell w-100 h-100 mx-2 px-2 py-2 rounded-xl my-1">
    <CalculatorStatus
      :payment-status="paymentStatus"
      :status-color="statusColor"
      :is-paid-in-full="isPaidInFull"
      :remaining-amount="remainingAmount"
      :total-summary="totalSummary"
    />

    <v-card class="calculator-display pa-3 mb-3" variant="outlined" rounded="lg">
      <span class="text-h5 font-weight-bold">{{ displayValue }}</span>
    </v-card>

    <CalculatorKeypad @digit="appendDigit" @backspace="backspace" @clear="clearInput" />
    <CalculatorPaymentMethods
      :methods="paymentMethods"
      :active-method="activeMethod"
      @select="selectMethodAndConfirm"
    />
    <CalculatorEntries
      :entries="entries"
      :format-amount="formatEntryAmount"
      :currency-symbol-for="currencySymbolFor"
      @remove="removeEntry"
    />
  </v-card>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { GetAppSettings, ListCurrencies, ListPaymentMethods } from "../../wailsjs/go/main/App";
import { formatUsd } from "@/lib/format";
import { roundLbpFromCents } from "@/lib/money";
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
const HALF_UP_LBP_TOLERANCE = 5000;
const HALF_DOWN_LBP_TOLERANCE = 10000;

const props = defineProps<{ total: number; roundingMode?: string }>();
const roundingMode = computed(() => props.roundingMode ?? "half_up");
const paymentMethods = ref<PaymentMethod[]>([]);
const currencies = ref<Currency[]>([]);
const exchangeRate = ref(DEFAULT_EXCHANGE_RATE);
const activeMethod = ref("");
const currentInput = ref("");
const entries = ref<Entry[]>([]);

const activeCurrency = computed(
  () => paymentMethods.value.find((m) => m.name === activeMethod.value)?.currencyCode ?? "",
);
const usdRate = computed(
  () => currencies.value.find((c) => c.code === "USD")?.valueInDefaultCurrency ?? DEFAULT_USD_RATE,
);
const activeRate = computed(() =>
  activeCurrency.value === "LBP"
    ? exchangeRate.value * usdRate.value
    : (currencies.value.find((c) => c.code === activeCurrency.value)?.valueInDefaultCurrency ?? 0),
);

function currencySymbolFor(methodName: string) {
  return paymentMethods.value.find((m) => m.name === methodName)?.currencyCode === "LBP"
    ? "LBP"
    : "$";
}

const totalCents = computed(() => Math.round(props.total * 100));
const paidCents = computed(() => entries.value.reduce((sum, entry) => sum + entry.amountCents, 0));
const diffCents = computed(() => paidCents.value - totalCents.value);
const diff = computed(() => diffCents.value / 100);
const isPaidInFull = computed(() => diffCents.value === 0);
const roundingToleranceCents = computed(() => {
  const stepLbp =
    roundingMode.value === "half_up" ? HALF_UP_LBP_TOLERANCE : HALF_DOWN_LBP_TOLERANCE;
  return Math.ceil((stepLbp * 100) / exchangeRate.value);
});
const checkoutReady = computed(() => Math.abs(diffCents.value) <= roundingToleranceCents.value);
const paymentStatus = computed(() =>
  isPaidInFull.value ? "Paid" : diffCents.value > 0 ? "Change to return" : "Amount to pay",
);
const statusColor = computed(() =>
  diff.value === 0
    ? "text-green-darken-2"
    : diff.value > 0
      ? "text-green-darken-1"
      : "text-red-darken-2",
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
function confirmEntry() {
  const amount = Number(currentInput.value);
  if (!Number.isFinite(amount) || amount <= 0 || activeRate.value <= 0 || usdRate.value <= 0)
    return;
  const method = paymentMethods.value.find((m) => m.name === activeMethod.value);
  const amountCents = Math.round((amount * usdRate.value * 100) / activeRate.value);
  if (amountCents <= 0) return;
  const isChange = diffCents.value > 0;
  entries.value.push({
    type: activeMethod.value,
    amount: isChange ? -amount : amount,
    amountCents: isChange ? -amountCents : amountCents,
    accountId: method?.id ?? 0,
  });
  currentInput.value = "";
}
function selectMethodAndConfirm(methodName: string) {
  activeMethod.value = methodName;
  confirmEntry();
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
  }));
  if (checkoutReady.value && diffCents.value !== 0) {
    for (let i = payments.length - 1; i >= 0; i -= 1) {
      if (payments[i]!.amountCents > 0) {
        payments[i]!.amountCents -= diffCents.value;
        break;
      }
    }
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
});
</script>

<style scoped>
.calculator-display {
  min-height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-color: rgba(var(--v-theme-on-surface), 0.18);
}
</style>
