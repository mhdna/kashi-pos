<template>
  <v-dialog :model-value="modelValue" max-width="460" @update:model-value="$emit('update:modelValue', $event)">
    <v-card title="Replace balance">
      <v-card-text>
        <div class="text-body-2 mb-3">
          Move value from <strong>{{ source?.accountName }}</strong>
          ({{ source?.currencyCode ?? "USD" }}) to another cash account.
        </div>
        <v-select v-model="sourceId" :items="sources" item-title="accountName" item-value="accountId" placeholder="From" variant="outlined" density="comfortable" class="mb-2" />
        <v-select v-model="destinationId" :items="destinations" item-title="name" item-value="id" placeholder="Replace with" variant="outlined" density="comfortable" class="mb-2" />
        <v-text-field v-model.number="amount" :placeholder="`Amount (${source?.currencyCode ?? 'USD'})`" type="number" min="0" :step="sourceCurrency && sourceCurrency.cashRoundingUnit > 1 ? sourceCurrency.cashRoundingUnit : 0.01" variant="outlined" density="comfortable" />
        <div v-if="destination && preview?.valid" class="text-caption text-medium-emphasis">
          Destination receives {{ formatBalance(preview.destinationReceivesCents, destination.currencyCode ?? "USD", currencies, roundingMode) }}
        </div>
        <div v-else-if="preview && !preview.valid && preview.reason" class="text-caption text-red-darken-2">
          {{ preview.reason }}
        </div>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="$emit('update:modelValue', false)">Cancel</v-btn>
        <v-btn color="primary" :loading="saving" :disabled="!valid" @click="save">Replace</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ReplaceShiftBalance, PreviewShiftBalanceReplacement, ConvertCashEntryToCents } from "../../../wailsjs/go/main/App";
import type { Balance, PaymentMethod } from "@/types/shifts";
import { formatBalance } from "@/lib/format";
import { useCurrencies, findCurrency } from "@/composables/useCurrencies";

const { currencies, roundingMode } = useCurrencies();

const props = defineProps<{
  modelValue: boolean;
  shiftId?: number;
  sources: Balance[];
  paymentMethods: PaymentMethod[];
}>();
const emit = defineEmits<{ "update:modelValue": [value: boolean]; saved: [shiftId: number] }>();
const destinationId = ref<number | null>(null);
const sourceId = ref<number | null>(null);
const amount = ref<number | null>(null);
const saving = ref(false);
const source = computed(() => props.sources.find((item) => item.accountId === sourceId.value) ?? null);
const sourceCurrency = computed(() => findCurrency(currencies.value, source.value?.currencyCode ?? "USD"));
const destinations = computed(() => props.paymentMethods.filter((method) => method.id !== source.value?.accountId));
const destination = computed(() => destinations.value.find((method) => method.id === destinationId.value));

function sourceAmount(balance: Balance | null): number {
  const value = Math.abs(Number(balance?.balance || 0));
  const currency = findCurrency(currencies.value, balance?.currencyCode ?? "USD");
  if (currency && currency.cashRoundingUnit > 1) {
    return Math.round((value * currency.unitsPerUsd) / 100 / currency.cashRoundingUnit) * currency.cashRoundingUnit;
  }
  return value / 100;
}

// entryCents/preview are both backend-computed: entryCents via the same
// conversion checkout uses (ConvertCashEntryToCents), and preview via the
// exact validation ReplaceShiftBalance itself applies
// (PreviewShiftBalanceReplacement) - so what this dialog shows can never
// diverge from what a submit actually does, unlike the old ad hoc,
// mode-less LBP math this replaces.
interface Preview {
  valid: boolean;
  reason: string;
  sourceRemainingCents: number;
  destinationReceivesCents: number;
}
const entryCents = ref(0);
const preview = ref<Preview | null>(null);
const valid = computed(() => !!props.shiftId && !!source.value && !!destination.value && Boolean(preview.value?.valid));

let refreshToken = 0;
async function refreshPreview() {
  const token = ++refreshToken;
  const amountValue = Number(amount.value);
  if (!source.value || !destinationId.value || !Number.isFinite(amountValue) || amountValue <= 0) {
    entryCents.value = 0;
    preview.value = null;
    return;
  }
  try {
    const cents = await ConvertCashEntryToCents(source.value.currencyCode ?? "USD", amountValue);
    if (token !== refreshToken) return;
    entryCents.value = Number(cents);
    if (!props.shiftId) {
      preview.value = null;
      return;
    }
    const result = await PreviewShiftBalanceReplacement(props.shiftId, source.value.accountId, destinationId.value, entryCents.value);
    if (token !== refreshToken) return;
    preview.value = {
      valid: Boolean(result.valid),
      reason: String(result.reason ?? ""),
      sourceRemainingCents: Number(result.sourceRemainingCents),
      destinationReceivesCents: Number(result.destinationReceivesCents),
    };
  } catch (error) {
    if (token !== refreshToken) return;
    entryCents.value = 0;
    preview.value = { valid: false, reason: error instanceof Error ? error.message : String(error), sourceRemainingCents: 0, destinationReceivesCents: 0 };
  }
}

let debounceTimer: ReturnType<typeof setTimeout> | undefined;
watch([amount, sourceId, destinationId], () => {
  clearTimeout(debounceTimer);
  debounceTimer = setTimeout(refreshPreview, 150);
});

watch(() => [props.modelValue, props.sources], ([open]) => {
  if (!open || !props.sources.length) return;
  const firstSource = props.sources[0];
  const physicalCashSource = props.sources.find((item) => {
    const currency = findCurrency(currencies.value, item.currencyCode ?? "USD");
    return currency && currency.cashRoundingUnit > 1;
  });
  sourceId.value = physicalCashSource?.accountId ?? firstSource?.accountId ?? null;
  destinationId.value = destinations.value[0]?.id ?? null;
  amount.value = sourceAmount(source.value);
});

watch(sourceId, () => {
  destinationId.value = destinations.value[0]?.id ?? null;
  amount.value = sourceAmount(source.value);
});

async function save() {
  if (!valid.value || !props.shiftId || !source.value || !destinationId.value) return;
  saving.value = true;
  try {
    await ReplaceShiftBalance(props.shiftId, source.value.accountId, destinationId.value, entryCents.value);
    emit("update:modelValue", false);
    emit("saved", props.shiftId);
  } catch (error) {
    window.alert(error instanceof Error ? error.message : String(error));
  } finally {
    saving.value = false;
  }
}
</script>
