<script setup lang="ts">
import { computed, inject } from "vue";
import { tabsKey } from "./keys";
import { colorVars } from "./util";

const props = defineProps<{ value?: unknown; text?: string }>();
const tabs = inject(tabsKey, null);
const selected = computed(() => tabs?.current() === props.value);
</script>
<template>
  <button class="k-tab" :class="{ 'k-tab--selected': selected }" :style="colorVars(tabs?.color)" type="button" role="tab" :aria-selected="selected" @click="tabs?.select(value)">
    <slot>{{ text }}</slot>
  </button>
</template>
