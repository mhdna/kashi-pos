<template>
  <div v-if="details.length" class="invoice-line-details" data-test="invoice-line-details">
    <span v-for="detail in details" :key="detail.key" class="invoice-line-detail" :data-test="`line-${detail.key}`">
      <span class="invoice-line-detail-label">{{ detail.label }}</span>
      <span class="invoice-line-detail-value">{{ detail.value }}</span>
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { invoiceLineDetails, type InvoiceLineProduct } from "@/lib/invoiceLineDetails";

const props = defineProps<{ line: InvoiceLineProduct | null | undefined }>();
const details = computed(() => invoiceLineDetails(props.line));
</script>

<style scoped>
.invoice-line-details {
  display: flex;
  flex-wrap: wrap;
  gap: 2px 14px;
  margin-top: 4px;
  font-size: 0.75rem;
  line-height: 1.35;
}
.invoice-line-detail {
  display: inline-flex;
  gap: 4px;
  white-space: nowrap;
}
.invoice-line-detail-label {
  opacity: 0.6;
}
.invoice-line-detail-value {
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}
</style>
