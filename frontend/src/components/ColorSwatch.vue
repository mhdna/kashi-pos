<script setup lang="ts">
// A round dot painted with a product's colour name (see lib/colorSwatch).
// Several colours share it in slices; an unknown name is an empty dashed dot.
import { computed } from "vue";
import { colourStops, swatchBackground } from "@/lib/colorSwatch";

const props = withDefaults(defineProps<{ name: string; size?: number }>(), { size: 16 });
const stops = computed(() => colourStops(props.name));
const style = computed(() => ({
  width: `${props.size}px`,
  height: `${props.size}px`,
  background: swatchBackground(stops.value),
}));
</script>

<template>
  <span
    class="color-swatch"
    :class="{ 'color-swatch--unknown': !stops.length }"
    :style="style"
    :title="name"
    :data-stops="stops.join(' ')"
    aria-hidden="true"
  />
</template>

<style scoped>
.color-swatch {
  display: inline-block;
  flex: none;
  border-radius: 50%;
  border: 1px solid rgba(var(--k-theme-on-surface), 0.28);
  box-sizing: border-box;
  vertical-align: middle;
}
.color-swatch--unknown {
  border-style: dashed;
  border-color: rgba(var(--k-theme-on-surface), 0.45);
}
</style>
