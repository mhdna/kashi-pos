<template>
  <section class="mb-4">
    <div class="calculator-amounts mb-2" :class="statusColor">
      <k-btn
        v-if="remainingAmount.amountsByCurrency.length > 1 && !showReadyState && !quoteError && !exchangeShortfallText"
        class="calculator-amounts-flip"
        icon="mdi-swap-vertical"
        variant="text"
        density="comfortable"
        size="small"
        title="Switch total currency"
        @click="flipCurrency"
      />
      <!-- A failed quote has no total to show. Rendering $0.00 here would
           read as a legitimate, fully-paid zero-value sale. -->
      <div v-if="quoteError" class="calculator-error-state">
        <k-icon icon="mdi-alert-circle" color="red-darken-2" size="28" />
        <span class="calculator-error-text" :title="quoteError">{{ quoteError }}</span>
      </div>
      <!-- An exchange that isn't covered yet is a normal step, not an error:
           a calm hint saying what's left to pick. -->
      <div v-else-if="exchangeShortfallText" class="calculator-error-state calculator-hint-state">
        <k-icon icon="mdi-information-outline" color="blue-grey-darken-1" size="28" />
        <span class="calculator-error-text">
          Add <strong class="calculator-hint-amount">{{ exchangeShortfallText }}</strong> more in new items to finish
          the exchange (no cash back)
        </span>
      </div>
      <div v-else-if="showReadyState" class="calculator-accepted-state calculator-green">
        <k-icon icon="mdi-check-circle" color="green-darken-3" size="40" />
        <span>{{ paymentStatus }}</span>
      </div>
      <template v-else>
        <!-- <span class="calculator-amounts-label" :class="statusColor">{{ paymentStatus }}</span> -->
        <div class="calculator-amount calculator-total-usd">
          <strong :class="statusColor">{{ displayedTotal }}</strong>
        </div>
        <div class="calculator-amounts-split">
          <span :class="statusColor">{{ remainingAmount.first }}</span>
          <span v-if="remainingAmount.second" class="calculator-amounts-plus" aria-hidden="true">+</span>
          <span v-if="remainingAmount.second" :class="statusColor">{{ remainingAmount.second }}</span>
        </div>
        <!-- kashi's whole-sale discount, already off the total above. -->
        <span v-if="invoiceDiscountText" class="calculator-amounts-invoice-discount" :title="invoiceDiscountText">
          {{ invoiceDiscountText }}
        </span>
        <!-- What it was rounded from is in the tooltip; the pill just says so. -->
        <span
          v-if="roundedFromText || isRounded"
          class="calculator-amounts-rounded"
          :title="roundedFromText || undefined"
        >
          Rounded
        </span>
      </template>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { calculatorTotalCurrency, setCalculatorTotalCurrency } from "@/lib/display";

const props = defineProps<{
  paymentStatus: string;
  statusColor: string;
  showReadyState: boolean;
  quoteError?: string | null;
  // Exchange mode only: cents of replacement value still needed to cover
  // the returned items (0/undefined otherwise). Exchanges must never pay
  // cash back to the customer, so this replaces the normal total/remaining
  // display with an explicit "add more" message instead of showing $0.00.
  exchangeShortfallCents?: number;
  // "Rounded from $5.6" when the backend rounded the total for cash.
  roundedFromText?: string;
  // "15% over $150 −$42": the sale's invoice discount, when it gets one.
  invoiceDiscountText?: string;
  remainingAmount: {
    // The two halves of the split, in display order -- which currency each
    // holds is per-branch config (see splitAmount in Calculator.vue).
    first: string;
    second: string;
    fullPrimary: string;
    fullSecondary: string;
    secondaryRounded: boolean;
    amountsByCurrency: { code: string; text: string; rounded: boolean }[];
  };
}>();

const exchangeShortfallText = computed(() => {
  const cents = props.exchangeShortfallCents ?? 0;
  if (cents <= 0) return "";
  return `$${(cents / 100).toFixed(2)}`;
});

const currentIndex = computed(() => {
  const index = props.remainingAmount.amountsByCurrency.findIndex(
    (c) => c.code === calculatorTotalCurrency.value,
  );
  return index === -1 ? 0 : index;
});

const displayedTotal = computed(
  () => props.remainingAmount.amountsByCurrency[currentIndex.value]?.text ?? props.remainingAmount.fullPrimary,
);
const isRounded = computed(
  () => Boolean(props.remainingAmount.amountsByCurrency[currentIndex.value]?.rounded) || props.remainingAmount.secondaryRounded,
);

function flipCurrency() {
  const list = props.remainingAmount.amountsByCurrency;
  if (list.length === 0) return;
  const next = list[(currentIndex.value + 1) % list.length];
  if (next) setCalculatorTotalCurrency(next.code);
}
</script>

<style scoped>
.calculator-amounts {
  position: relative;
  display: grid;
  grid-template-rows: repeat(2, minmax(0, 1fr));
  height: 66px;
  align-items: center;
  padding: 8px 12px;
  border: 1px solid rgba(var(--k-border-color), 0.12);
  border-radius: 10px;
  font-size: var(--kashi-font-size-base);
}
.calculator-amounts.calculator-green {
  color: #2e7d32;
  border-color: rgba(46, 125, 50, 0.5);
}
.calculator-green {
  color: #2e7d32;
}
.calculator-amounts.text-red-darken-2 {
  border-color: rgba(244, 67, 54, 0.5);
}
.calculator-amounts-flip {
  position: absolute;
  top: 4px;
  right: 4px;
  z-index: 1;
}
.calculator-amounts-rounded {
  position: absolute;
  bottom: 6px;
  left: 8px;
  padding: 3px 8px;
  border-radius: 999px;
  /* A tint of the status colour (green/red...) behind text in that colour. */
  background: color-mix(in srgb, currentColor 16%, transparent);
  font-size: var(--kashi-font-size-xs, 0.6875rem);
  font-weight: 600;
  line-height: 1;
}
.calculator-amounts-invoice-discount {
  position: absolute;
  bottom: 6px;
  right: 8px;
  max-width: 60%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding: 3px 8px;
  border-radius: 999px;
  background: rgba(46, 125, 50, 0.14);
  color: #2e7d32;
  font-size: var(--kashi-font-size-xs, 0.6875rem);
  font-weight: 600;
  line-height: 1;
}
.calculator-amounts-label {
  grid-row: 1;
  line-height: 1;
  text-align: center;
  font-size: var(--kashi-font-size-base);
  font-weight: 500;
}
.calculator-amount {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}
.calculator-total-usd { grid-row: 1; }
.calculator-total-lbp { grid-row: 2; }
.calculator-error-state {
  grid-row: 1 / -1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-width: 0;
  color: #c62828;
  font-size: var(--kashi-font-size-base);
  font-weight: 600;
}
.calculator-hint-state {
  color: rgba(var(--k-theme-on-surface), 0.8);
  font-weight: 500;
}
.calculator-amounts .calculator-hint-amount {
  font-size: inherit;
  font-weight: 700;
}
.calculator-error-text {
  min-width: 0;
  overflow: hidden;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-height: 1.2;
}
.calculator-accepted-state {
  grid-row: 1 / -1;
  display: flex;
  align-items: center;
  flex-direction: row;
  justify-content: center;
  gap: 8px;
  font-size: var(--kashi-font-size-base);
  font-weight: 700;
}
.calculator-amounts strong {
  padding: 0;
  text-align: center;
  font-variant-numeric: tabular-nums;
  font-size: var(--kashi-font-size-2xl);
}
.calculator-amounts-plus {
  color: rgba(var(--k-theme-on-surface), 0.55);
  font-weight: 700;
}
.calculator-amounts-split {
  grid-row: 2;
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 8px;
  font-variant-numeric: tabular-nums;
  font-size: var(--kashi-font-size-base);
  font-weight: 600;
  opacity: 1;
}
</style>
