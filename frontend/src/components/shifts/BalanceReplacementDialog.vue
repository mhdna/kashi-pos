<template>
  <v-dialog :model-value="modelValue" max-width="460" @update:model-value="$emit('update:modelValue', $event)">
    <v-card title="Replace balance">
      <v-card-text>
        <div class="text-body-2 mb-3">
          Move value from <strong>{{ source?.accountName }}</strong>
          ({{ source?.currencyCode ?? "USD" }}) to another cash account.
        </div>
        <v-select v-model="sourceId" :items="sources" item-title="accountName" item-value="accountId" label="From" variant="outlined" density="comfortable" class="mb-2" />
        <v-select v-model="destinationId" :items="destinations" item-title="name" item-value="id" label="Replace with" variant="outlined" density="comfortable" class="mb-2" />
        <v-text-field v-model.number="amount" :label="`Amount (${source?.currencyCode ?? 'USD'})`" type="number" min="0" :step="source?.currencyCode === 'LBP' ? 1000 : 0.01" variant="outlined" density="comfortable" />
        <div v-if="destination" class="text-caption text-medium-emphasis">
          Destination receives {{ formatBalance(destinationCents, destination.currencyCode ?? "USD", exchangeRate) }}
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
import { ReplaceShiftBalance } from "../../../wailsjs/go/main/App";
import type { Balance, PaymentMethod } from "@/types/shifts";
import { formatBalance } from "@/lib/format";
import { roundCentsFromLbp } from "@/lib/money";

const props = defineProps<{
  modelValue: boolean;
  shiftId?: number;
  sources: Balance[];
  paymentMethods: PaymentMethod[];
  exchangeRate: number;
}>();
const emit = defineEmits<{ "update:modelValue": [value: boolean]; saved: [shiftId: number] }>();
const destinationId = ref<number | null>(null);
const sourceId = ref<number | null>(null);
const amount = ref<number | null>(null);
const saving = ref(false);
const source = computed(() => props.sources.find((item) => item.accountId === sourceId.value) ?? null);
const destinations = computed(() => props.paymentMethods.filter((method) => method.id !== source.value?.accountId));
const destination = computed(() => destinations.value.find((method) => method.id === destinationId.value));
const cents = computed(() => {
  const value = Number(amount.value);
  if (!Number.isFinite(value) || value <= 0) return 0;
  return source.value?.currencyCode === "LBP" ? roundCentsFromLbp(value, props.exchangeRate) : Math.round(value * 100);
});
const destinationCents = computed(() => Number(source.value?.balance || 0) < 0 ? -cents.value : cents.value);
const valid = computed(() => !!props.shiftId && !!source.value && !!destination.value && cents.value > 0 && cents.value <= Math.abs(Number(source.value?.balance || 0)));

function sourceAmount(balance: Balance | null): number {
  const value = Math.abs(Number(balance?.balance || 0));
  return balance?.currencyCode === "LBP"
    ? Math.round(value * props.exchangeRate / 100)
    : value / 100;
}

watch(() => [props.modelValue, props.sources], ([open]) => {
  if (!open || !props.sources.length) return;
  const firstSource = props.sources[0];
  sourceId.value = props.sources.find((item) => item.currencyCode === "LBP")?.accountId ?? firstSource?.accountId ?? null;
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
    await ReplaceShiftBalance(props.shiftId, source.value.accountId, destinationId.value, cents.value);
    emit("update:modelValue", false);
    emit("saved", props.shiftId);
  } catch (error) {
    window.alert(error instanceof Error ? error.message : String(error));
  } finally {
    saving.value = false;
  }
}
</script>
