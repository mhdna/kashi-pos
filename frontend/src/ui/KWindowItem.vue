<script setup lang="ts">
import { computed, inject } from "vue";
import { tabsKey } from "./keys";

const props = defineProps<{ value?: unknown }>();
const window = inject(tabsKey, null);
const active = computed(() => window?.current() === props.value);
</script>
<template><div v-if="active" class="k-window-item"><slot /></div></template>
