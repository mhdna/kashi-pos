<template>
  <v-card class="calculator-shell w-100 h-100 mx-2 px-2 py-2 rounded-xl my-1">
    <!-- Progress -->
    <div class="mb-4">
      <div class="d-flex justify-space-between align-center mb-1">
        <span class="text-caption font-weight-medium" :class="statusColor">
          {{ paymentStatus }}
        </span>
        <span v-if="diffCents === 0" class="calculator-paid-state">
          <v-icon icon="mdi-check-circle" size="18" />
          <span>Ready to check out</span>
        </span>
      </div>
      <div v-if="diffCents !== 0" class="calculator-amounts mb-2">
        <div>
          <strong :class="statusColor">{{ statusRows.usd }}</strong>
        </div>
        <span class="calculator-amounts-plus" aria-hidden="true">+</span>
        <div>
          <strong :class="statusColor">{{ statusRows.lbp }}</strong>
        </div>
      </div>
    </div>

    <!-- Amount display -->
    <v-card class="calculator-display pa-3 mb-3" variant="outlined" rounded="lg">
      <span class="text-h5 font-weight-bold">{{ displayValue }}</span>
    </v-card>

    <!-- Keypad -->
    <div class="calculator-keypad">
      <div class="keypad-numbers">
        <v-btn
          v-for="n in [1, 2, 3, 4, 5, 6, 7, 8, 9]"
          :key="n"
          class="keypad-button keypad-number text-h6"
          @click="input(n.toString())"
        >
          {{ n }}
        </v-btn>
        <v-btn class="keypad-button keypad-number text-h6" variant="tonal" @click="input('.')"
          >.</v-btn
        >
        <v-btn class="keypad-button keypad-number text-h6" @click="input('0')">0</v-btn>
        <v-btn class="keypad-button keypad-number text-h6" @click="input('00')">00</v-btn>
      </div>
      <div class="keypad-actions">
        <v-btn
          class="keypad-button keypad-action text-h6"
          aria-label="Backspace"
          @click="input('back')"
        >
          ⌫
        </v-btn>
        <v-btn class="keypad-button keypad-action text-h6" color="red-darken-4" @click="clearInput">
          CA
        </v-btn>
      </div>
    </div>

    <!-- Payment methods also confirm the entered amount. -->
    <div class="calculator-types mt-3" role="list" aria-label="Payment methods">
      <v-card
        v-for="t in types"
        :key="t.name"
        class="calculator-type-card"
        :class="{ 'calculator-type-card--active': activeType === t.name }"
        role="button"
        tabindex="0"
        @click="selectAndConfirm(t.name)"
        @keydown.enter="selectAndConfirm(t.name)"
        @keydown.space.prevent="selectAndConfirm(t.name)"
      >
        <v-card-text class="text-center py-3 px-2">
          <div class="text-subtitle-2 font-weight-bold">{{ t.displayName }}</div>
          <div class="text-caption text-medium-emphasis">{{ t.currencyCode }}</div>
        </v-card-text>
      </v-card>
    </div>

    <!-- Entries -->
    <template v-if="entries.length">
      <v-divider class="my-3" />
      <v-virtual-scroll :items="entries" height="160" item-height="30">
        <template #default="{ item, index }">
          <div class="d-flex align-center justify-space-between py-0 px-1">
            <span class="calculator-entry-text text-caption">{{ item.type }}</span>
            <span
              class="calculator-entry-text text-caption font-weight-bold mr-2"
              :class="item.amount >= 0 ? 'text-green-darken-3' : 'text-red-darken-4'"
            >
              {{ item.amount >= 0 ? "+" : "" }}{{ formatEntryAmount(item.amount) }}
              {{ getCurrencySymbol(item.type) }}
            </span>
            <v-btn icon size="x-small" variant="text" @click="removeEntry(index)">
              <v-icon icon="mdi-delete" size="16" />
            </v-btn>
          </div>
        </template>
      </v-virtual-scroll>
    </template>
  </v-card>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from "vue";
import { GetAppSettings, ListCurrencies, ListPaymentMethods } from "../../wailsjs/go/main/App";
import { roundLbpFromCents } from "@/lib/money";

const props = defineProps<{ total: number; roundingMode?: string }>();

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

const types = ref<PaymentMethod[]>([]);
const currencies = ref<Currency[]>([]);
const activeType = ref("");
const entries = ref<{ type: string; amount: number; amountCents: number; accountId: number }[]>([]);
const currentInput = ref("");
const exchangeRate = ref(90000);
const roundingMode = computed(() => props.roundingMode ?? "half_up");

const activeCurrency = computed(
  () => types.value.find((t) => t.name === activeType.value)?.currencyCode ?? "",
);
const displayValue = computed(() => {
  if (!currentInput.value) return "0";
  return Number(currentInput.value).toLocaleString();
});

const activeRate = computed(() =>
  activeCurrency.value === "LBP"
    ? exchangeRate.value * usdRate.value
    : (currencies.value.find((c) => c.code === activeCurrency.value)?.valueInDefaultCurrency ?? 0),
);
const usdRate = computed(
  () => currencies.value.find((c) => c.code === "USD")?.valueInDefaultCurrency ?? 100,
);
const totalCents = computed(() => Math.round(props.total * 100));
const paidCents = computed(() => entries.value.reduce((sum, e) => sum + e.amountCents, 0));

const diffCents = computed(() => paidCents.value - totalCents.value);
const diff = computed(() => diffCents.value / 100);
// A rounded 10,000-LBP amount can differ from the exact amount by at most
// half of 10,000 LBP. Convert that allowance to accounting cents.
const roundingToleranceCents = computed(() =>
  Math.ceil(((roundingMode.value === "half_up" ? 5000 : 10000) * 100) / exchangeRate.value),
);
const checkoutReady = computed(() => Math.abs(diffCents.value) <= roundingToleranceCents.value);

const paymentStatus = computed(() => {
  if (diffCents.value === 0) return "Nothing remaining — paid in full";
  return diffCents.value > 0 ? "Change to return" : "Amount to pay";
});

const statusRows = computed(() => {
  const amountCents = Math.abs(diffCents.value);
  const dollarAmount = amountCents / 100;
  const wholeDollars = Math.floor(dollarAmount / 10) * 10;
  const remainderCents = amountCents - wholeDollars * 100;
  const roundedLbp = roundLbpFromCents(remainderCents, exchangeRate.value, roundingMode.value);
  return {
    usd: `$${wholeDollars.toLocaleString()}`,
    lbp: `L.L. ${roundedLbp.toLocaleString()}`,
  };
});

const statusColor = computed(() => {
  if (diff.value === 0) return "text-green-darken-2";
  return diff.value > 0 ? "text-green-darken-1" : "text-red-darken-2";
});

function getCurrencySymbol(type: string): string {
  const currencyCode = types.value.find((t) => t.name === type)?.currencyCode;
  return currencyCode === "LBP" ? "LBP" : "$";
}

function formatEntryAmount(amount: number): string {
  return amount.toLocaleString(undefined, { maximumFractionDigits: 2 });
}

onMounted(async () => {
  try {
    const settings = await GetAppSettings();
    const configuredRate = Number(settings?.exchangeRate);
    if (Number.isFinite(configuredRate) && configuredRate > 0) exchangeRate.value = configuredRate;
  } catch {}
  try {
    const methods = await ListPaymentMethods();
    types.value = methods.map((m: any) => ({
      id: m.id,
      name: m.name,
      displayName: m.displayName ?? m.name,
      currencyCode: m.currencyCode,
    }));
    if (types.value.length) activeType.value = types.value[0]!.name;
  } catch {}
  try {
    currencies.value = (await ListCurrencies()).map((c: any) => ({
      code: c.code,
      valueInDefaultCurrency: Number(c.valueInDefaultCurrency),
    }));
  } catch {}
});

function input(val: string) {
  if (val === "back") {
    currentInput.value = currentInput.value.slice(0, -1);
    return;
  }
  if (val === "." && currentInput.value.includes(".")) return;
  currentInput.value += val;
}

function confirm() {
  const amount = Number(currentInput.value);
  if (!Number.isFinite(amount) || amount <= 0 || activeRate.value <= 0 || usdRate.value <= 0)
    return;
  const method = types.value.find((t) => t.name === activeType.value);
  const amountCents = Math.round((amount * usdRate.value * 100) / activeRate.value);
  if (amountCents <= 0) return;
  // Once the customer has overpaid, new amounts represent change being
  // returned, so subtract them from the paid total instead of adding them.
  const signedAmount = diffCents.value > 0 ? -amount : amount;
  const signedAmountCents = diffCents.value > 0 ? -amountCents : amountCents;
  entries.value.push({
    type: activeType.value,
    amount: signedAmount,
    amountCents: signedAmountCents,
    accountId: method?.id ?? 0,
  });
  currentInput.value = "";
}

function selectAndConfirm(typeName: string) {
  activeType.value = typeName;
  confirm();
}

function clearInput() {
  currentInput.value = "";
}

function removeEntry(i: number) {
  entries.value.splice(i, 1);
}

function clearAll() {
  currentInput.value = "";
  entries.value = [];
  activeType.value = types.value.find((type) => type.currencyCode === "USD")?.name ?? "USD";
}

const firstEntryAccountId = computed(() => entries.value[0]?.accountId ?? 0);
const paymentsJSON = computed(() => {
  const payments = entries.value.map((entry) => ({
    accountId: entry.accountId,
    amountCents: entry.amountCents,
    entryType: entry.amountCents < 0 ? "change" : "payment",
  }));

  // Rounded LBP tender is displayed to the cashier, while checkout still
  // records an invoice-balanced amount in USD cents.
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

defineExpose({ diff, checkoutReady, clearAll, firstEntryAccountId, paymentsJSON });
</script>

<style scoped>
.calculator-keypad {
  display: grid;
  grid-template-columns: minmax(0, 3fr) minmax(0, 1fr);
  column-gap: 18px;
}

.keypad-numbers {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 6px;
}

.keypad-actions {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.keypad-button {
  min-height: 44px;
  border-radius: 12px;
  font-variant-numeric: tabular-nums;
}

.keypad-number {
  min-height: 40px;
}

.keypad-action {
  min-height: 0;
  flex: 1;
}

.calculator-display {
  min-height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-color: rgba(var(--v-theme-on-surface), 0.18);
}

.calculator-paid-state {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: rgb(var(--v-theme-primary));
  font-size: 0.78rem;
  font-weight: 700;
}

.calculator-entry-text {
  font-size: 0.85rem;
  line-height: 1.1;
}

.calculator-types {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(0, 1fr));
  gap: 8px;
}

.calculator-type-card {
  cursor: pointer;
  border: 1px solid rgba(var(--v-border-color), 0.18);
  transition:
    border-color 120ms ease,
    background-color 120ms ease;
}

.calculator-type-card:hover,
.calculator-type-card--active {
  border-color: rgb(var(--v-theme-primary));
  background-color: rgba(var(--v-theme-primary), 0.08);
}

.calculator-amounts {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
  min-height: 56px;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  border: 1px solid rgba(var(--v-border-color), 0.12);
  border-radius: 10px;
  font-size: 0.8rem;
}

.calculator-amounts > div {
  display: flex;
  flex-direction: column;
  gap: 2px;
  align-items: center;
}

.calculator-amounts-plus {
  align-self: center;
  color: rgba(var(--v-theme-on-surface), 0.55);
  font-size: 1.1rem;
  font-weight: 700;
}

.calculator-amounts strong {
  padding: 0;
  text-align: center;
  font-variant-numeric: tabular-nums;
  font-size: 0.95rem;
}
</style>
