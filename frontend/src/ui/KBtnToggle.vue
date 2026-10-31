<script setup lang="ts">
import { provide } from "vue";
import { btnDefaultsKey, btnToggleKey } from "./keys";

const props = defineProps<{ modelValue?: unknown; mandatory?: boolean; divided?: boolean; variant?: string; color?: string; density?: string; disabled?: boolean }>();
const emit = defineEmits<{ "update:modelValue": [value: any] }>();

provide(btnDefaultsKey, { variant: props.variant, color: props.color, density: props.density });
provide(btnToggleKey, {
  isSelected: (value) => props.modelValue === value,
  select: (value) => {
    if (props.disabled) return;
    if (props.modelValue === value) {
      if (!props.mandatory) emit("update:modelValue", undefined);
      return;
    }
    emit("update:modelValue", value);
  },
});
</script>
<template>
  <div class="k-btn-toggle" :class="[divided && 'k-btn-toggle--divided', props.density && `k-btn-toggle--density-${props.density}`]" role="group"><slot /></div>
</template>
