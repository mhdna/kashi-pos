<template>
  <div
    class="calculator-types mt-0"
    :class="{ 'calculator-types--vertical': vertical }"
    role="list"
    aria-label="Payment methods"
  >
    <v-card
      v-for="method in methods"
      :key="method.name"
      class="calculator-type-card"
      :class="{ 'calculator-type-card--active': modelValue === method.name }"
      :style="{ '--payment-accent': paymentAccentColor(method) }"
      elevation="0"
      role="button"
      tabindex="0"
      @click="$emit('select', method.name)"
      @keydown.enter="$emit('select', method.name)"
      @keydown.space.prevent="$emit('select', method.name)"
    >
      <v-card-text class="calculator-type-card-text text-center py-2 px-1">
        <div class="payment-method-name text-h6 font-weight-bold">{{ method.displayName }}</div>
        <div v-if="props.showCurrency" class="payment-method-currency text-body-1 text-medium-emphasis">
          {{ method.currencyCode }}
        </div>
      </v-card-text>
    </v-card>
  </div>
</template>

<script setup lang="ts">
import { paymentAccentColor } from "@/lib/paymentColor";

const props = withDefaults(
  defineProps<{
    methods: { name: string; displayName: string; currencyCode: string; color?: string }[];
    modelValue: string;
    vertical?: boolean;
    showCurrency?: boolean;
  }>(),
  { showCurrency: true },
);
defineEmits<{ select: [methodName: string] }>();
</script>

<style scoped>
.calculator-types {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(0, 1fr));
  gap: 8px;
}
.calculator-types--vertical {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  grid-auto-rows: 62px;
  max-height: calc(62px * 3 + 8px * 2);
  overflow-y: auto;
  padding-right: 2px;
}
.calculator-type-card {
  cursor: pointer;
  background: transparent !important;
  box-shadow: none !important;
  border: 1px solid rgba(var(--v-border-color), 0.18);
  border-top: 10px solid var(--payment-accent);
  height: 62px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: border-color 120ms ease;
}
.calculator-type-card-text {
  width: 100%;
  flex: 0 0 auto;
}
.payment-method-name {
  width: 100%;
  font-size: 1.1rem !important;
  line-height: 1.25;
  color: rgb(var(--v-theme-on-surface));
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.payment-method-currency {
  font-size: 0.9rem !important;
  min-height: 18px;
}
.calculator-type-card--active {
  border-color: var(--payment-accent);
}
</style>
