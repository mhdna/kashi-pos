<script setup lang="ts">
import { computed } from "vue";
import { colorVars, roundedStyle, toUnit } from "./util";
import KIcon from "./KIcon";
const props = defineProps<{ color?: string; variant?: string; size?: string | number; rounded?: string | boolean; icon?: string; text?: string }>();
const style = computed(() => {
  const s: Record<string, string | undefined> = { ...colorVars(props.color) };
  const size = toUnit(props.size);
  if (size) s["--k-av"] = size;
  const r = roundedStyle(props.rounded);
  if (r) s.borderRadius = r;
  return s;
});
</script>
<template>
  <span class="k-avatar" :class="[props.color && 'k-avatar--colored', `k-avatar--variant-${props.variant ?? 'flat'}`]" :style="style">
    <slot><KIcon v-if="icon" :icon="icon" /><template v-else>{{ text }}</template></slot>
  </span>
</template>
