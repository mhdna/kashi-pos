<template>
  <v-card class="calculator-total-summary pa-3 mb-3" variant="outlined" rounded="lg">
    <div class="d-flex justify-space-between text-body-1">
      <span class="text-medium-emphasis">Total USD:</span>
      <span class="calculator-total-value">{{ totalSummary.usd }}</span>
    </div>
    <div class="d-flex justify-space-between text-body-1">
      <span class="text-medium-emphasis">Total LBP:</span>
      <span class="calculator-total-value">{{ totalSummary.lbp }}</span>
    </div>
    <!-- <div class="d-flex justify-space-between text-caption"> -->
    <!--   <span class="text-medium-emphasis">Split:</span> -->
    <!--   <span>{{ totalSummary.split }}</span> -->
    <!-- </div> -->
  </v-card>

  <section class="mb-4">
    <div class="calculator-amounts mb-2">
      <div v-if="showReadyState" class="calculator-accepted-state text-green-darken-2">
        <v-icon icon="mdi-check-circle" color="green-darken-2" size="40" />
        <span>{{ paymentStatus }}</span>
      </div>
      <template v-else>
        <span class="calculator-amounts-label" :class="statusColor">{{ paymentStatus }}</span>
        <div class="calculator-amount calculator-amount-usd">
          <strong :class="statusColor">{{ remainingAmount.usd }}</strong>
        </div>
        <span class="calculator-amounts-plus" aria-hidden="true">+</span>
        <div class="calculator-amount calculator-amount-lbp">
          <strong :class="statusColor">{{ remainingAmount.lbp }}</strong>
        </div>
      </template>
    </div>
  </section>
</template>

<script setup lang="ts">
defineProps<{
  paymentStatus: string;
  statusColor: string;
  showReadyState: boolean;
  remainingAmount: { usd: string; lbp: string };
  totalSummary: { usd: string; lbp: string; split: string };
}>();
</script>

<style scoped>
.calculator-total-summary {
  border-color: rgba(var(--v-theme-on-surface), 0.18);
  font-size: 1.1rem;
}
.calculator-total-summary .text-medium-emphasis {
  font-size: 1.1rem !important;
}
.calculator-total-value {
  color: #1a1a1a;
  font-weight: 600;
  font-size: 1.1rem;
  line-height: 1.2;
}
.calculator-amounts {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
  height: 56px;
  align-items: center;
  column-gap: 12px;
  row-gap: 2px;
  padding: 8px 12px;
  border: 1px solid rgba(var(--v-border-color), 0.12);
  border-radius: 10px;
  font-size: var(--kashi-font-size-base);
}
.calculator-amounts-label {
  grid-column: 1 / -1;
  line-height: 1;
  text-align: center;
  font-size: var(--kashi-font-size-sm);
  font-weight: 500;
}
.calculator-amount {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}
.calculator-amount-usd { grid-column: 1; }
.calculator-amount-lbp { grid-column: 3; }
.calculator-accepted-state {
  grid-column: 1 / -1;
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
  font-size: 1.1rem;
}
.calculator-amounts-plus {
  align-self: center;
  color: rgba(var(--v-theme-on-surface), 0.55);
  font-size: var(--kashi-font-size-lg);
  font-weight: 700;
}
</style>
