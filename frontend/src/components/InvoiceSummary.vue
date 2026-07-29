<template>
  <div class="mx-2 pa-3 mb-2" color="grey-darken-4">
    <div class="d-flex justify-space-between text-caption">
      <span class="text-medium-emphasis">Subtotal</span>
      <span>{{ formatUsd(subtotal) }}</span>
    </div>
    <div class="d-flex justify-space-between text-caption">
      <span class="text-medium-emphasis">Discount</span>
      <span class="text-red">-{{ formatUsd(totalDiscount) }}</span>
    </div>
    <div class="d-flex justify-space-between text-caption">
      <span class="text-medium-emphasis">After Discount</span>
      <span>{{ formatUsd(afterDiscount) }}</span>
    </div>
    <!-- <v-divider class="my-1" /> -->
    <div class="d-flex justify-space-between text-caption">
      <span class="text-medium-emphasis">VAT ({{ (taxRate * 100).toFixed(0) }}%)</span>
      <span>{{ formatUsd(vat) }}</span>
    </div>
    <!-- <v-divider class="my-1" /> -->
    <!-- <div class="d-flex justify-space-between text-body-2 font-weight-bold"> -->
    <!--   <span>Total</span> -->
    <!--   <span>{{ formatUsd(total) }}</span> -->
    <!-- </div> -->
  </div>
</template>

<script lang="ts" setup>
import { computed } from "vue";
import { formatUsd } from "@/lib/format";

const props = defineProps<{
  subtotal: number;
  totalDiscount: number;
  taxRate: number;
}>();

const afterDiscount = computed(() => props.subtotal - props.totalDiscount);
const vat = computed(() => afterDiscount.value * props.taxRate);
const total = computed(() => afterDiscount.value + vat.value);
</script>
