<template>
  <template v-if="entries.length">
    <k-divider class="my-3" />
    <div class="calculator-entry-grid">
      <div
        v-for="(item, index) in entries"
        :key="index"
        class="calculator-entry-chip d-flex align-center justify-space-between"
      >
        <span class="calculator-entry-text text-caption">{{ item.type }}</span>
        <span
          class="calculator-entry-text text-caption font-weight-bold mr-1"
          :class="item.amount >= 0 ? 'text-green-darken-3' : 'text-red-darken-4'"
        >
          {{ item.amount >= 0 ? "+" : "" }}{{ formatAmount(item.amount) }}
          {{ currencySymbolFor(item.type) }}
        </span>
        <k-btn icon size="x-small" variant="text" @click="$emit('remove', index)"
          ><k-icon icon="mdi-delete" size="16"
        /></k-btn>
      </div>
    </div>
  </template>
</template>

<script setup lang="ts">
defineProps<{
  entries: { type: string; amount: number }[];
  formatAmount: (amount: number) => string;
  currencySymbolFor: (methodName: string) => string;
}>();
defineEmits<{ remove: [index: number] }>();
</script>

<style scoped>
.calculator-entry-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  max-height: 168px;
  overflow-y: auto;
  scrollbar-gutter: stable;
}
.calculator-entry-chip {
  min-height: 36px;
  padding: 4px 10px;
  border-radius: 12px;
  background: rgba(var(--k-theme-on-surface), 0.03);
  border: 1px solid rgba(var(--k-border-color), 0.16);
}
.calculator-entry-text {
  font-size: var(--kashi-font-size-sm);
  line-height: 1.1;
}
</style>
