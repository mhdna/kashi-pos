<script setup lang="ts" generic="T">
import { computed, ref } from "vue";
import { useScrollWindow } from "./useScrollWindow";
import { toUnit } from "./util";
import { visibleRange } from "./virtual";

const props = defineProps<{ items: readonly T[]; itemHeight: number | string; height?: string | number; maxHeight?: string | number }>();
defineSlots<{ default(scope: { item: T; index: number }): unknown }>();

const root = ref<HTMLElement | null>(null);
const { scrollTop, height: viewport } = useScrollWindow(root);
const rowHeight = computed(() => Number(props.itemHeight));
const range = computed(() => visibleRange(scrollTop.value, viewport.value, rowHeight.value, props.items.length));
const rows = computed(() => props.items.slice(range.value.start, range.value.end).map((item, i) => ({ item, index: range.value.start + i })));

function scrollToIndex(index: number) {
  if (root.value) root.value.scrollTop = Math.max(0, index) * rowHeight.value;
}
defineExpose({ scrollToIndex });
</script>

<template>
  <div ref="root" class="k-virtual-scroll" :style="{ height: toUnit(height), maxHeight: toUnit(maxHeight) }">
    <div :style="{ height: `${range.padTop}px` }" />
    <div v-for="row in rows" :key="row.index" class="k-virtual-scroll__item" :style="{ height: `${rowHeight}px` }"><slot :item="row.item" :index="row.index" /></div>
    <div :style="{ height: `${range.padBottom}px` }" />
  </div>
</template>
