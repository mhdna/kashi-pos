<template>
  <v-sheet class="mx-2 mb-2 pa-3" color="grey-darken-4" rounded>
    <div class="d-flex justify-space-between text-caption">
      <span class="text-medium-emphasis">Subtotal</span>
      <span>${{ subtotal.toFixed(2) }}</span>
    </div>
    <div class="d-flex justify-space-between text-caption">
      <span class="text-medium-emphasis">Discount</span>
      <span class="text-red">-${{ totalDiscount.toFixed(2) }}</span>
    </div>
    <div class="d-flex justify-space-between text-caption">
      <span class="text-medium-emphasis">After Discount</span>
      <span>${{ afterDiscount.toFixed(2) }}</span>
    </div>
    <v-divider class="my-1" />
    <div class="d-flex justify-space-between text-caption">
      <span class="text-medium-emphasis">VAT (11%)</span>
      <span>${{ vat.toFixed(2) }}</span>
    </div>
    <v-divider class="my-1" />
    <div class="d-flex justify-space-between text-body-2 font-weight-bold">
      <span>Total</span>
      <span>${{ total.toFixed(2) }}</span>
    </div>
  </v-sheet>
</template>

<script lang="ts" setup>
import { computed } from "vue";

const props = defineProps<{
  subtotal: number;
  totalDiscount: number;
}>();

const afterDiscount = computed(() => props.subtotal - props.totalDiscount);
const vat = computed(() => afterDiscount.value * 0.11);
const total = computed(() => afterDiscount.value + vat.value);
</script>
