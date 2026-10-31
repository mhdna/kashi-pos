<script setup lang="ts">
import { colorVars } from "./util";
const props = defineProps<{ modelValue?: number; min?: number | string; max?: number | string; step?: number | string; color?: string; showTicks?: boolean | string; hideDetails?: boolean | string; disabled?: boolean }>();
const emit = defineEmits<{ "update:modelValue": [value: number] }>();
</script>
<template>
  <div class="k-slider" :style="colorVars(color)">
    <input type="range" :min="min ?? 0" :max="max ?? 100" :step="step ?? 1" :value="modelValue ?? 0" :disabled="disabled" @input="emit('update:modelValue', Number(($event.target as HTMLInputElement).value))" />
  </div>
</template>
