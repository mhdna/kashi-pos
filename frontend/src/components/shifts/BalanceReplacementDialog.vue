<template>
  <k-dialog :model-value="modelValue" max-width="560" @update:model-value="$emit('update:modelValue', $event)">
    <k-card class="swap-dialog">
      <div class="swap-dialog__header">
        <k-avatar color="primary" variant="tonal" size="40">
          <k-icon icon="mdi-swap-horizontal" />
        </k-avatar>
        <div>
          <div class="text-h6">{{ transfer ? "Transfer balance" : "Swap balance" }}</div>
          <div class="text-body-2 text-medium-emphasis">
            {{ transfer ? "Take an amount out of one account, even below zero." : "Move money from one cash account to another." }}
          </div>
        </div>
      </div>

      <k-card-text class="pt-2">
        <k-btn-toggle v-if="auth.isAdmin" v-model="mode" mandatory divided variant="outlined" color="primary" density="comfortable" class="swap-dialog__mode mb-4" data-test="swap-mode">
          <k-btn value="swap" prepend-icon="mdi-swap-horizontal">Swap</k-btn>
          <k-btn value="transfer" prepend-icon="mdi-arrow-right">Transfer</k-btn>
        </k-btn-toggle>

        <div class="swap-dialog__label">From</div>
        <div class="swap-dialog__tiles" data-test="swap-sources">
          <button
            v-for="item in sources"
            :key="item.accountId"
            type="button"
            class="swap-tile"
            :class="{ 'swap-tile--selected': item.accountId === sourceId }"
            :aria-pressed="item.accountId === sourceId"
            data-test="swap-source"
            @click="sourceId = item.accountId"
          >
            <k-avatar :color="paymentAccentColor({ name: item.accountName, color: item.color })" variant="tonal" size="36">
              <k-icon :icon="paymentIcon(item.accountName)" size="20" />
            </k-avatar>
            <span class="swap-tile__text">
              <span class="swap-tile__name">{{ item.accountName }}</span>
              <span class="swap-tile__balance" :class="{ 'text-error': Number(item.balance) < 0 }">
                {{ formatBalance(item.balance, item.currencyCode ?? "USD", currencies, roundingMode) }}
              </span>
            </span>
            <k-icon :icon="currencyIcon(item.currencyCode ?? 'USD')" size="18" class="swap-tile__currency" :title="item.currencyCode ?? 'USD'" />
          </button>
        </div>

        <div class="swap-dialog__arrow" aria-hidden="true">
          <k-icon icon="mdi-arrow-down" size="20" />
        </div>

        <div class="swap-dialog__label">To</div>
        <div class="swap-dialog__tiles" data-test="swap-destinations">
          <button
            v-for="method in destinations"
            :key="method.id"
            type="button"
            class="swap-tile"
            :class="{ 'swap-tile--selected': method.id === destinationId }"
            :aria-pressed="method.id === destinationId"
            data-test="swap-destination"
            @click="destinationId = method.id"
          >
            <k-avatar :color="paymentAccentColor({ name: method.name })" variant="tonal" size="36">
              <k-icon :icon="paymentIcon(method.name)" size="20" />
            </k-avatar>
            <span class="swap-tile__text">
              <span class="swap-tile__name">{{ method.displayName || method.name }}</span>
              <span class="swap-tile__balance">{{ method.currencyCode ?? "USD" }}</span>
            </span>
            <k-icon :icon="currencyIcon(method.currencyCode ?? 'USD')" size="18" class="swap-tile__currency" />
          </button>
        </div>

        <div class="swap-dialog__label mt-4">Amount</div>
        <div class="swap-dialog__amount">
          <k-text-field
            v-model.number="amount"
            :prepend-inner-icon="currencyIcon(sourceCode)"
            :prefix="sourceIsNegative && !transfer ? '−' : undefined"
            :suffix="sourceCurrency?.symbol && sourceCode !== 'USD' ? sourceCurrency.symbol : undefined"
            :placeholder="`Amount (${sourceCode})`"
            type="number"
            min="0"
            :step="sourceCurrency && sourceCurrency.cashRoundingUnit > 1 ? sourceCurrency.cashRoundingUnit : 0.01"
            variant="outlined"
            density="comfortable"
            hide-details
            data-test="swap-amount"
          />
          <k-btn v-if="source && !transfer" variant="tonal" size="large" data-test="swap-all" @click="fillAll">All</k-btn>
        </div>

        <div v-if="source && destination && preview?.valid" class="swap-summary" data-test="swap-summary">
          <k-icon icon="mdi-check-circle" color="success" size="20" />
          <div>
            <div>
              <strong>{{ destination.displayName || destination.name }}</strong> receives
              <strong>{{ formatBalance(preview.destinationReceivesCents, destination.currencyCode ?? "USD", currencies, roundingMode) }}</strong>
            </div>
            <div v-if="transfer" class="text-medium-emphasis">
              {{ source.accountName }} is left with {{ formatBalance(preview.sourceRemainingCents, sourceCode, currencies, roundingMode) }}
            </div>
          </div>
        </div>
        <div v-else-if="preview && !preview.valid && preview.reason" class="swap-summary swap-summary--error" data-test="swap-error">
          <k-icon icon="mdi-alert-circle-outline" color="error" size="20" />
          <div>{{ preview.reason }}</div>
        </div>
      </k-card-text>
      <k-card-actions class="px-6 pb-4">
        <k-spacer />
        <k-btn variant="text" @click="$emit('update:modelValue', false)">Cancel</k-btn>
        <k-btn color="primary" variant="flat" :prepend-icon="transfer ? 'mdi-arrow-right' : 'mdi-swap-horizontal'" :loading="saving" :disabled="!valid" data-test="swap-save" @click="save">
          {{ transfer ? "Transfer" : "Swap" }}
        </k-btn>
      </k-card-actions>
    </k-card>
  </k-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ReplaceShiftBalance, PreviewShiftBalanceReplacement, ConvertCashEntryToCents } from "../../../wailsjs/go/main/App";
import type { Balance, PaymentMethod } from "@/types/shifts";
import { formatBalance } from "@/lib/format";
import { currencyIcon } from "@/lib/currencyIcon";
import { paymentAccentColor, paymentIcon } from "@/lib/paymentColor";
import { useCurrencies, findCurrency } from "@/composables/useCurrencies";
import { useAuthStore } from "@/stores/auth";

const { currencies, roundingMode } = useCurrencies();
const auth = useAuthStore();

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
// Transfer mode (admin only, re-checked by the backend) moves exactly the
// entered amount and may leave the source below zero; a swap can't.
const transfer = ref(false);
const saving = ref(false);
const source = computed(() => props.sources.find((item) => item.accountId === sourceId.value) ?? null);
const sourceCode = computed(() => source.value?.currencyCode ?? "USD");
const sourceCurrency = computed(() => findCurrency(currencies.value, sourceCode.value));
// The Swap/Transfer switch (admins only) drives `transfer`.
const mode = computed({
  get: () => (transfer.value ? "transfer" : "swap"),
  set: (value: string) => (transfer.value = value === "transfer"),
});
const sourceIsNegative = computed(() => Number(source.value?.balance ?? 0) < 0);
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
    const result = await PreviewShiftBalanceReplacement(props.shiftId, source.value.accountId, destinationId.value, entryCents.value, transfer.value);
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
watch([amount, sourceId, destinationId, transfer], () => {
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
  transfer.value = false;
  sourceId.value = physicalCashSource?.accountId ?? firstSource?.accountId ?? null;
  destinationId.value = destinations.value[0]?.id ?? null;
  amount.value = sourceAmount(source.value);
});

watch(sourceId, () => {
  destinationId.value = destinations.value[0]?.id ?? null;
  amount.value = transfer.value ? null : sourceAmount(source.value);
});

// A transfer's amount has nothing to do with the source's balance, so don't
// prefill it with that.
watch(transfer, (on) => {
  amount.value = on ? null : sourceAmount(source.value);
});

// "All" moves the source's whole balance, rounded to its cash unit.
function fillAll() {
  amount.value = sourceAmount(source.value);
}

async function save() {
  if (!valid.value || !props.shiftId || !source.value || !destinationId.value) return;
  saving.value = true;
  try {
    await ReplaceShiftBalance(props.shiftId, source.value.accountId, destinationId.value, entryCents.value, transfer.value);
    emit("update:modelValue", false);
    emit("saved", props.shiftId);
  } catch (error) {
    window.alert(error instanceof Error ? error.message : String(error));
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped>
.swap-dialog__header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 20px 24px 8px;
}
.swap-dialog__mode {
  width: 100%;
}
.swap-dialog__mode :deep(.k-btn) {
  flex: 1 1 0;
}
.swap-dialog__label {
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: rgba(var(--k-theme-on-surface), 0.6);
  margin-bottom: 6px;
}
.swap-dialog__tiles {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 8px;
}
.swap-tile {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 60px;
  padding: 8px 10px;
  border: 1px solid rgba(var(--k-theme-on-surface), 0.14);
  border-radius: 10px;
  background: rgb(var(--k-theme-surface));
  color: inherit;
  text-align: start;
  cursor: pointer;
  transition: border-color 0.15s, background-color 0.15s;
}
.swap-tile:hover {
  background: rgba(var(--k-theme-on-surface), 0.04);
}
.swap-tile--selected {
  border: 2px solid rgb(var(--k-theme-primary));
  padding: 7px 9px;
  background: rgba(var(--k-theme-primary), 0.06);
}
.swap-tile__text {
  display: flex;
  flex-direction: column;
  min-width: 0;
  flex: 1 1 auto;
}
.swap-tile__name {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.swap-tile__balance {
  font-size: 0.8125rem;
  color: rgba(var(--k-theme-on-surface), 0.65);
  font-variant-numeric: tabular-nums;
}
.swap-tile__currency {
  opacity: 0.5;
  flex: none;
}
.swap-dialog__arrow {
  display: flex;
  justify-content: center;
  margin: 8px 0 2px;
  color: rgba(var(--k-theme-on-surface), 0.5);
}
.swap-dialog__amount {
  display: flex;
  align-items: center;
  gap: 8px;
}
.swap-dialog__amount > :first-child {
  flex: 1 1 auto;
}
.swap-summary {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  margin-top: 12px;
  padding: 10px 12px;
  border-radius: 10px;
  background: rgba(var(--k-theme-success), 0.08);
  font-size: 0.875rem;
}
.swap-summary--error {
  background: rgba(var(--k-theme-error), 0.08);
  color: rgb(var(--k-theme-error));
}
</style>
