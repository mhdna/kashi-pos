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
      <!-- Down the left side, one under the other: the invoice's own total (only
           while the big number is something else: what is left to pay, the
           change, "Paid"), then "Rounded", then the discounts. -->
      <div v-if="leftCount" class="calculator-amounts-left" data-test="card-left">
        <span v-if="totalText" class="calculator-amounts-total" data-test="invoice-total">{{ totalText }}</span>
        <template v-if="showPills">
          <!-- What it was rounded from is in the tooltip; the pill just says so. -->
          <span v-if="showRounded" class="calculator-amounts-rounded" :title="roundedFromText || undefined">Rounded</span>
          <!-- central's whole-sale discount, already off the total. -->
          <span v-if="invoiceDiscountText" class="calculator-amounts-discount" :title="invoiceDiscountText">
            {{ invoiceDiscountText }}
          </span>
          <!-- Everything taken off the sale in all: lines, deals and that. -->
          <span v-if="discountText" class="calculator-amounts-discount" data-test="discount-total">
            {{ discountText }}
          </span>
        </template>
      </div>
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
        <div class="calculator-amount calculator-total-usd" :class="{ 'calculator-total-alone': !showSplit && !otherTotalText }">
          <strong :class="statusColor">{{ displayedTotal }}</strong>
        </div>
        <div v-if="showSplit" class="calculator-amounts-split">
          <span :class="statusColor">{{ remainingAmount.first }}</span>
          <span v-if="remainingAmount.second" class="calculator-amounts-plus" aria-hidden="true">+</span>
          <span v-if="remainingAmount.second" :class="statusColor">{{ remainingAmount.second }}</span>
        </div>
        <!-- When the cash split would only repeat the total, the same amount in
             the other currency goes there instead, so both views of the card
             have two lines. -->
        <div v-else-if="otherTotalText" class="calculator-amounts-split calculator-amounts-other">
          <span :class="statusColor">{{ otherTotalText }}</span>
        </div>
      </template>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { calculatorTotalCurrency, setCalculatorTotalCurrency } from "@/lib/display";
import { splitRepeatsTotal } from "@/lib/totalSplit";

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
  // "Discount −$42.00": everything taken off the sale in all.
  discountText?: string;
  // "Total $250.00": the invoice's total, given only when the card's own
  // number is not it.
  totalText?: string;
  remainingAmount: {
    // The two halves of the split, in display order -- which currency each
    // holds is per-branch config (see splitAmount in Calculator.vue).
    first: string;
    second: string;
    // Currency of `first` when it stands alone (no second half).
    firstCode?: string;
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
const displayedCode = computed(
  () => props.remainingAmount.amountsByCurrency[currentIndex.value]?.code ?? "USD",
);
// "$30.00" over "$30" says the same thing twice; the split line only shows
// when it adds something (a second half, or another currency).
const showSplit = computed(() => !splitRepeatsTotal(props.remainingAmount, displayedCode.value));
const isRounded = computed(
  () => Boolean(props.remainingAmount.amountsByCurrency[currentIndex.value]?.rounded) || props.remainingAmount.secondaryRounded,
);

// The same total in the currency the flip button would switch to; "" on a
// till with one currency.
const otherTotalText = computed(() => {
  const list = props.remainingAmount.amountsByCurrency;
  if (list.length < 2) return "";
  return list[(currentIndex.value + 1) % list.length]?.text ?? "";
});

// The pills show on the plain total only, not over an error, a hint or "Paid".
const showPills = computed(() => !props.quoteError && !exchangeShortfallText.value && !props.showReadyState);
const showRounded = computed(() => Boolean(props.roundedFromText) || isRounded.value);
// How many chips the left side holds (they stack, and wrap into a second
// column when there are more than fit; the card keeps its height).
const leftCount = computed(
  () =>
    (props.totalText ? 1 : 0) +
    (showPills.value
      ? (showRounded.value ? 1 : 0) + (props.invoiceDiscountText ? 1 : 0) + (props.discountText ? 1 : 0)
      : 0),
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
  border-color: #2e7d32;
}
.calculator-green {
  color: #2e7d32;
}
.calculator-amounts.text-red-darken-2 {
  border-color: #c62828;
}
.calculator-amounts-flip {
  position: absolute;
  top: 4px;
  right: 4px;
  z-index: 1;
}
.calculator-amounts-left {
  position: absolute;
  top: 6px;
  bottom: 6px;
  left: 10px;
  /* Clear of the numbers in the middle; a long name is cut with an ellipsis. */
  max-width: 44%;
  display: flex;
  flex-direction: column;
  flex-wrap: wrap;
  /* Packed against the bottom of the card, whatever number of chips there are. */
  justify-content: flex-end;
  align-content: flex-start;
  align-items: flex-start;
  gap: 3px 6px;
}
/* The invoice total is a chip like the rest, in a neutral tone. */
.calculator-amounts-total {
  flex: 0 0 auto;
  padding: 2px 8px;
  border-radius: 999px;
  background: rgba(var(--k-theme-on-surface), 0.08);
  color: rgba(var(--k-theme-on-surface), 0.75);
  font-size: var(--kashi-font-size-xs, 0.6875rem);
  font-weight: 600;
  line-height: 1.2;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.calculator-amounts-rounded {
  flex: 0 0 auto;
  padding: 2px 8px;
  border-radius: 999px;
  /* A tint of the status colour (green/red...) behind text in that colour. */
  background: color-mix(in srgb, currentColor 16%, transparent);
  font-size: var(--kashi-font-size-xs, 0.6875rem);
  font-weight: 600;
  line-height: 1.2;
}
.calculator-amounts-discount {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding: 2px 8px;
  border-radius: 999px;
  /* Red: it is money taken off. The theme's error colour, so it also reads in
     the dark theme. */
  background: color-mix(in srgb, rgb(var(--k-theme-error)) 14%, transparent);
  color: rgb(var(--k-theme-error));
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
/* No split line under it: centre the total in the whole card. */
.calculator-total-alone { grid-row: 1 / -1; justify-content: center; }
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
