<template>
  <template v-if="entries.length">
    <v-divider class="my-3" />
    <v-virtual-scroll :items="entries" height="160" item-height="30">
      <template #default="{ item, index }">
        <div class="d-flex align-center justify-space-between py-0 px-1">
          <span class="calculator-entry-text text-caption">{{ item.type }}</span>
          <span
            class="calculator-entry-text text-caption font-weight-bold mr-2"
            :class="item.amount >= 0 ? 'text-green-darken-3' : 'text-red-darken-4'"
          >
            {{ item.amount >= 0 ? "+" : "" }}{{ formatAmount(item.amount) }}
            {{ currencySymbolFor(item.type) }}
          </span>
          <v-btn icon size="x-small" variant="text" @click="$emit('remove', index)"
            ><v-icon icon="mdi-delete" size="16"
          /></v-btn>
        </div>
      </template>
    </v-virtual-scroll>
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
.calculator-entry-text {
  font-size: 0.85rem;
  line-height: 1.1;
}
</style>
