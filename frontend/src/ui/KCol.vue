<script setup lang="ts">
import { computed } from "vue";
const props = defineProps<{ cols?: string | number }>();
const size = computed(() => (props.cols && props.cols !== "auto" && !Number.isNaN(Number(props.cols)) ? Number(props.cols) : undefined));
</script>
<template>
  <div class="k-col" :class="{ 'k-col--sized': size !== undefined, 'k-col--auto': cols === 'auto' }" :style="size !== undefined ? { '--k-col-size': size } : undefined"><slot /></div>
</template>
