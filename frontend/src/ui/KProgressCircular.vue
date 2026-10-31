<script setup lang="ts">
import { computed } from "vue";
import { colorVars, toUnit } from "./util";
const props = withDefaults(defineProps<{ indeterminate?: boolean; modelValue?: number | string; size?: string | number; width?: string | number; color?: string }>(), { width: 4, modelValue: 0 });
const radius = 20;
const circumference = 2 * Math.PI * radius;
const style = computed(() => {
  const s: Record<string, string | undefined> = { ...colorVars(props.color) };
  const size = toUnit(props.size ?? 48);
  s["--k-pc-size"] = size;
  return s;
});
const strokeWidth = computed(() => (Number(props.width) / (Number(props.size ?? 48) || 48)) * 44);
const dash = computed(() => `${(Number(props.modelValue) / 100) * circumference} ${circumference}`);
</script>
<template>
  <span class="k-progress-circular" :class="{ 'k-progress-circular--indeterminate': indeterminate }" :style="style" role="progressbar">
    <svg viewBox="0 0 44 44">
      <circle class="k-progress-circular__underlay" cx="22" cy="22" :r="radius" :stroke-width="strokeWidth" />
      <circle class="k-progress-circular__overlay" cx="22" cy="22" :r="radius" :stroke-width="strokeWidth" :stroke-dasharray="indeterminate ? undefined : dash" />
    </svg>
  </span>
</template>
