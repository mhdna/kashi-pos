<script setup lang="ts">
import { colorVars } from "./util";
import KProgressCircular from "./KProgressCircular.vue";

const props = defineProps<{ modelValue?: boolean | null; label?: string; color?: string; loading?: boolean; disabled?: boolean; hideDetails?: boolean | string; hint?: string; persistentHint?: boolean; density?: string }>();
const emit = defineEmits<{ "update:modelValue": [value: boolean] }>();
</script>
<template>
  <div class="k-input k-switch" :class="[props.density && props.density !== 'default' && `k-input--density-${props.density}`]" :style="{ flex: '0 1 auto' }">
    <label class="k-selection k-switch-wrap" :class="[modelValue && 'k-switch--on', disabled && 'k-selection--disabled']" :style="colorVars(color)">
      <input class="k-selection__input" type="checkbox" role="switch" :checked="!!modelValue" :disabled="disabled" @change="emit('update:modelValue', ($event.target as HTMLInputElement).checked)" />
      <span class="k-switch__track"><span class="k-switch__thumb"><KProgressCircular v-if="loading" indeterminate size="14" :width="2" /></span></span>
      <span v-if="label || $slots.label" class="k-selection__label"><slot name="label">{{ label }}</slot></span>
    </label>
    <div v-if="hideDetails !== true && hint && (persistentHint)" class="k-input__details">{{ hint }}</div>
  </div>
</template>
