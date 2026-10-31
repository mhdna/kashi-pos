<script setup lang="ts">
import { provide } from "vue";
import { tabsKey } from "./keys";

const props = defineProps<{ modelValue?: unknown; color?: string; grow?: boolean; density?: string }>();
const emit = defineEmits<{ "update:modelValue": [value: unknown] }>();
provide(tabsKey, { current: () => props.modelValue, select: (v) => emit("update:modelValue", v), color: props.color });
</script>
<template>
  <div class="k-tabs" :class="[grow && 'k-tabs--grow', density && density !== 'default' && `k-tabs--density-${density}`]" role="tablist"><slot /></div>
</template>
