<template>
  <!-- How an exchange comes to its total, under the cart: every row is the
       backend's (lib/exchangeQuote.ts), and the rows add up to "To pay". -->
  <div class="exchange-summary" data-test="exchange-summary">
    <div
      v-for="row in rows"
      :key="row.key"
      class="exchange-summary-row"
      :class="{
        'exchange-summary-row--note': row.note,
        'exchange-summary-row--total': row.total,
      }"
      :data-row="row.key"
    >
      <span class="exchange-summary-label">{{ row.label }}</span>
      <span v-if="!row.note" class="exchange-summary-amount">{{ formatCents(row.cents) }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { exchangeSummaryRows, formatCents, type ExchangeQuoteResult } from "@/lib/exchangeQuote";

const props = defineProps<{ quote: ExchangeQuoteResult }>();
const rows = computed(() => exchangeSummaryRows(props.quote));
</script>

<style scoped>
.exchange-summary {
  border: 1px solid rgba(var(--k-theme-on-surface), 0.12);
  border-radius: 8px;
  margin: 4px 8px;
  padding: 6px 10px;
  font-size: 0.85rem;
}

.exchange-summary-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  line-height: 1.6;
}

.exchange-summary-amount {
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.exchange-summary-row--note {
  color: rgba(var(--k-theme-on-surface), 0.65);
  font-size: 0.78rem;
  padding-inline-start: 10px;
}

.exchange-summary-row--total {
  border-top: 1px solid rgba(var(--k-theme-on-surface), 0.12);
  font-weight: 700;
  margin-top: 2px;
  padding-top: 2px;
}
</style>
