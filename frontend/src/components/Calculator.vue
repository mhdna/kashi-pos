<template>
  <div class="w-100 h-100 px-4 py-2 rounded-xl">
    <!-- Payment type -->
    <v-btn-toggle v-model="activeType" class="w-100 mb-2" divided mandatory density="comfortable">
      <v-btn v-for="t in types" :key="t.name" class="flex-grow-1" size="small" :value="t.name">
        {{ t.name }}
      </v-btn>
    </v-btn-toggle>

    <!-- Amount display -->
    <v-card class="pa-4 mb-4 text-right" variant="outlined" rounded="lg">
      <span class="text-h5 font-weight-bold">{{ displayValue }}</span>
      <span class="text-body-2 text-medium-emphasis ml-1">{{ activeType }}</span>
    </v-card>

    <!-- Keypad -->
    <v-row>
      <v-col v-for="n in [1, 2, 3, 4, 5, 6, 7, 8, 9]" :key="n" cols="4">
        <v-btn block class="text-h6" @click="input(n.toString())">{{ n }}</v-btn>
      </v-col>
      <v-col cols="4"><v-btn block class="text-h6" @click="input('.')">.</v-btn></v-col>
      <v-col cols="4"><v-btn block class="text-h6" @click="input('0')">0</v-btn></v-col>
      <v-col cols="4"><v-btn block class="text-h6" @click="input('00')">00</v-btn></v-col>
      <v-col cols="6">
        <v-btn block class="text-h6" color="red-darken-2" @click="clearAll">CA</v-btn>
      </v-col>
      <v-col cols="6">
        <v-btn block class="text-h6" @click="input('back')">⌫</v-btn>
      </v-col>
    </v-row>

    <!-- Confirm -->
    <v-btn
      block
      class="mt-4 text-h6 font-weight-bold"
      :color="diffCents > 0 ? 'grey-darken-1' : 'grey-darken-4'"
      :disabled="!currentInput || total === 0"
      height="44"
      @click="confirm"
    >
      {{ diffCents > 0 ? "− Subtract payment" : "+ Add payment" }}
    </v-btn>

    <!-- Progress -->
    <div class="mt-6">
      <div class="d-flex justify-space-between align-center mb-1">
        <span class="text-caption text-medium-emphasis">{{ statusLabel }}</span>
        <span class="text-body-2 font-weight-medium text-right" :class="statusColor">
          <span v-for="line in statusLines" :key="line" class="d-block">{{ line }}</span>
        </span>
      </div>
      <v-progress-linear
        bg-color="grey-darken-3"
        :color="progressColor"
        striped
        height="10"
        :model-value="progressPct"
        rounded
      />
    </div>

    <!-- Entries -->
    <template v-if="entries.length">
      <v-divider class="my-4" />
      <div
        v-for="(e, i) in entries"
        :key="i"
        class="d-flex align-center justify-space-between py-2 px-1"
      >
        <span class="text-body-2">{{ e.type }}</span>
        <span
          class="text-body-1 font-weight-bold mr-2"
          :class="e.amount >= 0 ? 'text-green-darken-3' : 'text-red-darken-4'"
        >
          {{ e.amount >= 0 ? "+" : "" }}{{ formatEntryAmount(e.amount) }}
          {{ getCurrencySymbol(e.type) }}
        </span>
        <v-btn icon size="small" variant="text" @click="removeEntry(i)">
          <v-icon icon="mdi-delete" size="18" />
        </v-btn>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from "vue";
import { GetAppSettings, ListCurrencies, ListPaymentMethods } from "../../wailsjs/go/main/App";
import { roundLbpFromCents } from "@/lib/money";

const props = defineProps<{ total: number; roundingMode?: string }>();

interface PaymentMethod {
  id: number;
  name: string;
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
const roundingToleranceCents = computed(() => Math.ceil(((roundingMode.value === "half_up" ? 5000 : 10000) * 100) / exchangeRate.value));
const checkoutReady = computed(() => Math.abs(diffCents.value) <= roundingToleranceCents.value);

const progressPct = computed(() =>
  Math.min(totalCents.value > 0 ? (paidCents.value / totalCents.value) * 100 : 0, 100),
);

const statusLabel = computed(() => {
  if (diffCents.value === 0) return "Paid in full";
  return diffCents.value > 0 ? "Change" : "Remaining";
});

const statusLines = computed(() => {
  if (diffCents.value === 0) return ["Exact"];
  const amountCents = Math.abs(diffCents.value);
  const dollarAmount = amountCents / 100;
  const wholeDollars = Math.floor(dollarAmount / 10) * 10;
  const remainderCents = amountCents - wholeDollars * 100;
  const roundedLbp = roundLbpFromCents(remainderCents, exchangeRate.value, roundingMode.value);
  const lines = [];
  if (wholeDollars > 0) lines.push(`${diffCents.value > 0 ? "+" : ""}${wholeDollars}$`);
  if (roundedLbp > 0) {
    // lines.push(`${roundedLbp.toLocaleString()} LBP${roundingLabel}${diffCents.value > 0 ? "" : " due"}`);
    lines.push(`${roundedLbp.toLocaleString()} LBP`);
  }
  return lines;
});

const statusColor = computed(() => {
  if (diff.value === 0) return "text-green-darken-2";
  return diff.value > 0 ? "text-green-darken-1" : "text-red-darken-2";
});

const progressColor = computed(() => {
  if (diff.value === 0) return "green-darken-2";
  return diff.value > 0 ? "green-darken-1" : "red-darken-2";
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

function removeEntry(i: number) {
  entries.value.splice(i, 1);
}

function clearAll() {
  currentInput.value = "";
  entries.value = [];
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
