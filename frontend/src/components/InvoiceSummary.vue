<template>
  <div class="mx-2 pa-3 mb-2" color="grey-darken-4">
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
      <span class="text-medium-emphasis">VAT ({{ (taxRate * 100).toFixed(0) }}%)</span>
      <span>${{ vat.toFixed(2) }}</span>
    </div>
    <v-divider class="my-1" />
    <div class="d-flex justify-space-between text-body-2 font-weight-bold">
      <span>Total</span>
      <span>${{ total.toFixed(2) }}</span>
    </div>
    <!-- <div v-if="total > 0" class="mt-2 text-center text-caption text-medium-emphasis">
      <span v-for="(c, i) in currencies" :key="c.code">
        <template v-if="i > 0"> · </template>
        {{ c.symbol }}
        {{ (total * (c.valueInDefaultCurrency / 100)).toFixed(2) }}
      </span>
    </div> -->
  </div>
</template>

<script lang="ts" setup>
import { computed, ref, onMounted } from "vue";
import { ListCurrencies } from "../../wailsjs/go/main/App";

const props = defineProps<{
  subtotal: number;
  totalDiscount: number;
  taxRate: number;
}>();

const currencies = ref<any[]>([]);

const afterDiscount = computed(() => props.subtotal - props.totalDiscount);
const vat = computed(() => afterDiscount.value * props.taxRate);
const total = computed(() => afterDiscount.value + vat.value);

onMounted(async () => {
  try {
    currencies.value = await ListCurrencies();
  } catch {}
});
</script>
