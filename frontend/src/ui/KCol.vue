<script setup lang="ts">
import { computed } from "vue";
const props = defineProps<{ cols?: string | number }>();
const style = computed(() => {
  const n = Number(props.cols);
  if (!props.cols || props.cols === "auto" || Number.isNaN(n)) return undefined;
  return { flex: `0 0 ${(n / 12) * 100}%`, maxWidth: `${(n / 12) * 100}%` };
});
</script>
<template><div class="k-col" :data-cols="cols === 'auto' ? 'auto' : undefined" :style="style"><slot /></div></template>
