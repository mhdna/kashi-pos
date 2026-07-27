<template>
  <v-card class="calculator-total-summary pa-3 mb-3" variant="outlined" rounded="lg">
    <div class="d-flex justify-space-between text-caption">
      <span class="text-medium-emphasis">Total USD:</span>
      <span>{{ totalSummary.usd }}</span>
    </div>
    <div class="d-flex justify-space-between text-caption">
      <span class="text-medium-emphasis">Total LBP:</span>
      <span>{{ totalSummary.lbp }}</span>
    </div>
    <!-- <div class="d-flex justify-space-between text-caption"> -->
    <!--   <span class="text-medium-emphasis">Split:</span> -->
    <!--   <span>{{ totalSummary.split }}</span> -->
    <!-- </div> -->
  </v-card>

  <section class="mb-4">
    <div class="d-flex justify-space-between align-center mb-1">
      <span class="text-caption font-weight-medium" :class="statusColor">{{ paymentStatus }}</span>
      <span v-if="isPaidInFull" class="calculator-paid-state">
        <v-icon icon="mdi-check-circle" size="18" />
        <span>Ready to check out</span>
      </span>
    </div>

    <div v-if="!isPaidInFull" class="calculator-amounts mb-2">
      <div>
        <strong :class="statusColor">{{ remainingAmount.usd }}</strong>
      </div>
      <span class="calculator-amounts-plus" aria-hidden="true">+</span>
      <div>
        <strong :class="statusColor">{{ remainingAmount.lbp }}</strong>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
defineProps<{
  paymentStatus: string;
  statusColor: string;
  isPaidInFull: boolean;
  remainingAmount: { usd: string; lbp: string };
  totalSummary: { usd: string; lbp: string; split: string };
}>();
</script>

<style scoped>
.calculator-paid-state {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: rgb(var(--v-theme-primary));
  font-size: 0.78rem;
  font-weight: 700;
}
.calculator-total-summary {
  border-color: rgba(var(--v-theme-on-surface), 0.18);
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
  align-items: center;
  gap: 2px;
}
.calculator-amounts strong {
  padding: 0;
  text-align: center;
  font-variant-numeric: tabular-nums;
  font-size: 0.95rem;
}
.calculator-amounts-plus {
  align-self: center;
  color: rgba(var(--v-theme-on-surface), 0.55);
  font-size: 1.1rem;
  font-weight: 700;
}
</style>
