<script setup lang="ts">
import { computed } from "vue";
import { colorVars, toUnit } from "./util";
const props = withDefaults(defineProps<{ indeterminate?: boolean; modelValue?: number | string; height?: string | number; color?: string; rounded?: boolean; absolute?: boolean; location?: string; active?: boolean }>(), { modelValue: 0, active: true });
const style = computed(() => ({ ...colorVars(props.color), "--k-pl-h": toUnit(props.height ?? 4) }));
</script>
<template>
  <div v-if="active" class="k-progress-linear" :class="{ 'k-progress-linear--absolute': absolute, 'k-progress-linear--top': location === 'top', 'k-progress-linear--rounded': rounded }" :style="style" role="progressbar">
    <div class="k-progress-linear__bg" />
    <div v-if="indeterminate" class="k-progress-linear__bar k-progress-linear__bar--ind" />
    <div v-else class="k-progress-linear__bar" :style="{ width: `${Math.min(100, Math.max(0, Number(modelValue)))}%` }" />
  </div>
</template>
