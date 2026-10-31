<script setup lang="ts">
import { computed, provide } from "vue";
import { listKey } from "./keys";
import { toUnit } from "./util";

const props = defineProps<{ density?: string; nav?: boolean; lines?: string | boolean; bgColor?: string; maxHeight?: string | number; minWidth?: string | number; border?: boolean | string; rounded?: boolean | string }>();
provide(listKey, { nav: !!props.nav, density: props.density ?? "default" });
const style = computed(() => ({
  maxHeight: toUnit(props.maxHeight),
  minWidth: toUnit(props.minWidth),
  background: props.bgColor === "transparent" ? "transparent" : undefined,
}));
</script>
<template>
  <div class="k-list" :class="[nav && 'k-list--nav', border !== undefined && border !== false && 'k-list--border', rounded !== undefined && rounded !== false && 'k-list--rounded']" :style="style" role="list">
    <slot />
  </div>
</template>
