<script setup lang="ts">
import { computed } from "vue";
import { roundedStyle } from "./util";
const props = defineProps<{ rounded?: string | boolean; border?: boolean | string; color?: string; tag?: string }>();
const radius = computed(() => roundedStyle(props.rounded));
</script>
<template>
  <component :is="tag || 'div'" class="k-sheet" :class="{ 'k-sheet--border': border !== undefined && border !== false }" :style="radius ? { borderRadius: radius } : undefined">
    <slot />
  </component>
</template>
