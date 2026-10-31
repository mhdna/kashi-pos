<script setup lang="ts">
import { colorVars } from "./util";
import KIcon from "./KIcon";

const props = defineProps<{ modelValue?: boolean | null; label?: string; color?: string; disabled?: boolean; hideDetails?: boolean | string; density?: string }>();
const emit = defineEmits<{ "update:modelValue": [value: boolean] }>();
</script>
<template>
  <div class="k-input k-checkbox" :style="{ flex: '0 1 auto' }">
    <label class="k-selection" :class="[modelValue && 'k-checkbox--on', disabled && 'k-selection--disabled']" :style="colorVars(color)">
      <input class="k-selection__input" type="checkbox" :checked="!!modelValue" :disabled="disabled" @change="emit('update:modelValue', ($event.target as HTMLInputElement).checked)" />
      <span class="k-checkbox__box"><KIcon v-if="modelValue" icon="mdi-check" /></span>
      <span v-if="label || $slots.label" class="k-selection__label"><slot name="label">{{ label }}</slot></span>
    </label>
  </div>
</template>
