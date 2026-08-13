<template>
  <template v-if="entries.length">
    <v-divider class="my-3" />
    <v-virtual-scroll
      ref="entryScroll"
      :items="entries"
      :height="listHeight"
      :item-height="ENTRY_HEIGHT"
      class="calculator-entry-scroll"
    >
      <template #default="{ item, index }">
        <div class="calculator-entry-row d-flex align-center justify-space-between px-1">
          <span class="calculator-entry-text text-caption">{{ item.type }}</span>
          <span
            class="calculator-entry-text calculator-entry-amount text-caption font-weight-bold mr-2"
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
import { computed, nextTick, ref, watch } from "vue";

const ENTRY_HEIGHT = 36;
const MAX_VISIBLE_ENTRIES = 4;

const props = defineProps<{
  entries: { type: string; amount: number }[];
  formatAmount: (amount: number) => string;
  currencySymbolFor: (methodName: string) => string;
}>();
defineEmits<{ remove: [index: number] }>();

const entryScroll = ref<{ scrollToIndex: (index: number) => void } | null>(null);
const listHeight = computed(
  () => Math.min(props.entries.length, MAX_VISIBLE_ENTRIES) * ENTRY_HEIGHT,
);

watch(
  () => props.entries.length,
  async (length) => {
    if (!length) return;
    await nextTick();
    entryScroll.value?.scrollToIndex(length - 1);
  },
);
</script>

<style scoped>
.calculator-entry-scroll {
  flex: 0 0 auto;
  overflow-y: auto;
  scrollbar-gutter: stable;
}
.calculator-entry-row {
  min-height: 36px;
}
.calculator-entry-text {
  font-size: var(--kashi-font-size-sm);
  line-height: 1.1;
}
.calculator-entry-amount {
  font-size: 1.3rem;
}
</style>
