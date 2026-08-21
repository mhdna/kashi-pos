<template>
  <section class="mb-4">
    <div class="calculator-amounts mb-2">
      <v-btn
        v-if="remainingAmount.amountsByCurrency.length > 1 && !showReadyState"
        class="calculator-amounts-flip"
        icon="mdi-swap-vertical"
        variant="text"
        density="comfortable"
        size="small"
        title="Switch total currency"
        @click="flipCurrency"
      />
      <div v-if="showReadyState" class="calculator-accepted-state text-green-darken-2">
        <v-icon icon="mdi-check-circle" color="green-darken-2" size="40" />
        <span>{{ paymentStatus }}</span>
      </div>
      <template v-else>
        <span class="calculator-amounts-label" :class="statusColor">{{ paymentStatus }}</span>
        <div class="calculator-amount calculator-total-usd">
          <strong :class="statusColor">{{ displayedTotal }}</strong>
        </div>
        <div class="calculator-amounts-split">
          <span :class="statusColor">{{ remainingAmount.usd }}</span>
          <span v-if="remainingAmount.lbp" class="calculator-amounts-plus" aria-hidden="true">+</span>
          <span v-if="remainingAmount.lbp" :class="statusColor">{{ remainingAmount.lbp }}</span>
        </div>
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
  remainingAmount: {
    usd: string;
    lbp: string;
    fullUsd: string;
    fullLbp: string;
    amountsByCurrency: { code: string; text: string }[];
  };
}>();

const currentIndex = computed(() => {
  const index = props.remainingAmount.amountsByCurrency.findIndex(
    (c) => c.code === calculatorTotalCurrency.value,
  );
  return index === -1 ? 0 : index;
});

const displayedTotal = computed(
  () => props.remainingAmount.amountsByCurrency[currentIndex.value]?.text ?? props.remainingAmount.fullUsd,
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
  grid-template-rows: repeat(3, minmax(0, 1fr));
  height: 100px;
  align-items: center;
  padding: 8px 12px;
  border: 1px solid rgba(var(--v-border-color), 0.12);
  border-radius: 10px;
  font-size: var(--kashi-font-size-base);
}
.calculator-amounts-flip {
  position: absolute;
  top: 4px;
  right: 4px;
  z-index: 1;
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
.calculator-total-usd { grid-row: 2; }
.calculator-total-lbp { grid-row: 3; }
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
  color: rgba(var(--v-theme-on-surface), 0.55);
  font-weight: 700;
}
.calculator-amounts-split {
  grid-row: 3;
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
