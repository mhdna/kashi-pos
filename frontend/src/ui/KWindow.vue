<script setup lang="ts">
import { provide } from "vue";
import { tabsKey } from "./keys";

const props = defineProps<{ modelValue?: unknown }>();
const emit = defineEmits<{ "update:modelValue": [value: unknown] }>();
provide(tabsKey, { current: () => props.modelValue, select: (v) => emit("update:modelValue", v) });
</script>
<template><div class="k-window"><slot /></div></template>
