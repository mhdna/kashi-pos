<script setup lang="ts">
import { computed } from "vue";
import KIcon from "./KIcon";

// The visual shell shared by text fields, textareas and selects: outline,
// floating label, inner icons, clear button and the hint/error row.
const props = defineProps<{
  variant?: string;
  density?: string;
  label?: string;
  focused?: boolean;
  floating?: boolean;
  error?: boolean;
  disabled?: boolean;
  multiline?: boolean;
  prependInnerIcon?: string;
  appendInnerIcon?: string;
  showClear?: boolean;
  message?: string;
  messageIsError?: boolean;
  hideDetails?: boolean | string;
  prefix?: string;
  suffix?: string;
  counter?: string;
}>();
const emit = defineEmits<{
  "click:prepend-inner": [event: MouseEvent];
  "click:append-inner": [event: MouseEvent];
  "click:clear": [event: MouseEvent];
}>();
defineSlots<{ default?(): unknown; "prepend-inner"?(): unknown; "append-inner"?(): unknown }>();

const variant = computed(() => props.variant ?? "filled");
const showDetails = computed(() => props.hideDetails !== true && (props.hideDetails !== "auto" || !!props.message));
</script>

<template>
  <div class="k-input" :class="[props.density && props.density !== 'default' && `k-input--density-${props.density}`, disabled && 'k-input--disabled']">
    <div
      class="k-field"
      :class="[
        `k-field--variant-${variant}`,
        focused && 'k-field--focused',
        error && 'k-field--error',
        floating && label && 'k-field--label-floating',
        multiline && 'k-field--textarea',
        (prependInnerIcon || $slots['prepend-inner']) && 'k-field--has-prepend',
        (appendInnerIcon || $slots['append-inner'] || showClear) && 'k-field--has-append',
      ]"
    >
      <span class="k-field__outline" />
      <div v-if="prependInnerIcon || $slots['prepend-inner']" class="k-field__prepend" @click="emit('click:prepend-inner', $event)">
        <slot name="prepend-inner"><KIcon :icon="prependInnerIcon" /></slot>
      </div>
      <div class="k-field__control">
        <label v-if="label" class="k-field__label">{{ label }}</label>
        <span v-if="prefix" class="k-field__affix">{{ prefix }}&nbsp;</span>
        <slot />
        <span v-if="suffix" class="k-field__affix">&nbsp;{{ suffix }}</span>
      </div>
      <div v-if="showClear" class="k-field__append k-field__clear">
        <KIcon icon="mdi-close-circle" class="k-field__icon-btn" @mousedown.prevent @click="emit('click:clear', $event)" />
      </div>
      <div v-if="appendInnerIcon || $slots['append-inner']" class="k-field__append" @click="emit('click:append-inner', $event)">
        <slot name="append-inner"><KIcon :icon="appendInnerIcon" :class="{ 'k-field__icon-btn': true }" /></slot>
      </div>
    </div>
    <div v-if="showDetails" class="k-input__details" :class="{ 'k-input__details--error': messageIsError }">
      <span>{{ message }}</span>
      <span v-if="counter">{{ counter }}</span>
    </div>
  </div>
</template>
