<template>
  <div class="calculator-types mt-3" role="list" aria-label="Payment methods">
    <v-card
      v-for="method in methods"
      :key="method.name"
      class="calculator-type-card"
      role="button"
      tabindex="0"
      @click="$emit('select', method.name)"
      @keydown.enter="$emit('select', method.name)"
      @keydown.space.prevent="$emit('select', method.name)"
    >
      <v-card-text class="text-center py-2 px-1">
        <div class="text-caption font-weight-bold">{{ method.displayName }}</div>
        <div class="text-caption text-medium-emphasis">{{ method.currencyCode }}</div>
      </v-card-text>
    </v-card>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  methods: { name: string; displayName: string; currencyCode: string }[];
  modelValue: string;
}>();
defineEmits<{ select: [methodName: string] }>();
</script>

<style scoped>
.calculator-types {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(0, 1fr));
  gap: 8px;
}
.calculator-type-card {
  cursor: pointer;
  border: 1px solid rgba(var(--v-border-color), 0.18);
  transition:
    border-color 120ms ease,
    background-color 120ms ease;
}
.calculator-type-card:hover {
  border-color: rgb(var(--v-theme-primary));
  background-color: rgba(var(--v-theme-primary), 0.08);
}
</style>
