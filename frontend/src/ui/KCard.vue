<script setup lang="ts">
import { computed, useAttrs } from "vue";
import { colorVars, roundedStyle, toUnit } from "./util";

const props = defineProps<{
  variant?: string;
  color?: string;
  rounded?: string | boolean;
  elevation?: string | number;
  title?: string;
  subtitle?: string;
  width?: string | number;
  maxWidth?: string | number;
  minWidth?: string | number;
  height?: string | number;
  maxHeight?: string | number;
  disabled?: boolean;
}>();
const attrs = useAttrs();
const slots = defineSlots<{ default?(): unknown; title?(): unknown; subtitle?(): unknown; prepend?(): unknown; append?(): unknown }>();

const variant = computed(() => props.variant ?? "elevated");
const hasItem = computed(() => !!(props.title || props.subtitle || slots.title || slots.subtitle || slots.prepend || slots.append));
const style = computed(() => {
  const s: Record<string, string | undefined> = { ...colorVars(props.color) };
  const r = roundedStyle(props.rounded);
  if (r) s.borderRadius = r;
  if (props.elevation !== undefined) s.boxShadow = `var(--k-elevation-${props.elevation})`;
  if (props.elevation !== undefined && Number(props.elevation) === 0) s.boxShadow = "none";
  s.width = toUnit(props.width);
  s.maxWidth = toUnit(props.maxWidth);
  s.minWidth = toUnit(props.minWidth);
  s.height = toUnit(props.height);
  s.maxHeight = toUnit(props.maxHeight);
  return s;
});
</script>

<template>
  <div
    class="k-card"
    :class="[variant !== 'elevated' && `k-card--${variant}`, color && 'k-card--colored', attrs.onClick && 'k-card--link']"
    :style="style"
  >
    <div v-if="hasItem" class="k-card-item">
      <div v-if="$slots.prepend" class="k-card-item__prepend"><slot name="prepend" /></div>
      <div class="k-card-item__content">
        <div v-if="title || $slots.title" class="k-card-title"><slot name="title">{{ title }}</slot></div>
        <div v-if="subtitle || $slots.subtitle" class="k-card-subtitle"><slot name="subtitle">{{ subtitle }}</slot></div>
      </div>
      <div v-if="$slots.append" class="k-card-item__append"><slot name="append" /></div>
    </div>
    <slot />
  </div>
</template>
