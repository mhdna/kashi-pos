<script setup lang="ts">
import { computed, inject } from "vue";
import KIcon from "./KIcon";
import KProgressCircular from "./KProgressCircular.vue";
import { btnDefaultsKey, btnToggleKey } from "./keys";
import { colorVars, namedSizes, roundedStyle, toUnit } from "./util";

const props = defineProps<{
  variant?: string;
  color?: string;
  size?: string | number;
  icon?: boolean | string;
  prependIcon?: string;
  appendIcon?: string;
  block?: boolean;
  loading?: boolean;
  disabled?: boolean;
  density?: string;
  rounded?: string | boolean;
  height?: string | number;
  width?: string | number;
  type?: "button" | "submit" | "reset";
  text?: string;
  value?: unknown;
  active?: boolean;
}>();

const defaults = inject(btnDefaultsKey, {});
const toggle = inject(btnToggleKey, null);

const variant = computed(() => props.variant ?? defaults.variant ?? "elevated");
const color = computed(() => props.color ?? defaults.color);
const density = computed(() => props.density ?? defaults.density ?? "default");
const isIcon = computed(() => props.icon !== undefined && props.icon !== false);
const named = computed(() => (props.size !== undefined && namedSizes.includes(String(props.size)) ? String(props.size) : undefined));
const isActive = computed(() => props.active || (toggle !== null && props.value !== undefined && toggle.isSelected(props.value)));

const style = computed(() => {
  const s: Record<string, string | undefined> = { ...colorVars(color.value) };
  if (props.size !== undefined && !named.value) {
    s.width = toUnit(props.size);
    s.height = toUnit(props.size);
  }
  if (props.height !== undefined) s.height = toUnit(props.height);
  if (props.width !== undefined) s.width = toUnit(props.width);
  const r = roundedStyle(props.rounded);
  if (r) s.borderRadius = r;
  return s;
});

function onClick() {
  if (toggle && props.value !== undefined) toggle.select(props.value);
}
</script>

<template>
  <button
    class="k-btn"
    :class="[
      `k-btn--variant-${variant}`,
      color && 'k-btn--colored',
      named && `k-btn--size-${named}`,
      density !== 'default' && `k-btn--density-${density}`,
      isIcon && 'k-btn--icon',
      props.size !== undefined && !named && 'k-btn--sized',
      block && 'k-btn--block',
      loading && 'k-btn--loading',
      disabled && 'k-btn--disabled',
      isActive && 'k-btn--active',
    ]"
    :style="style"
    :type="type ?? 'button'"
    :disabled="disabled || undefined"
    @click="onClick"
  >
    <span v-if="loading" class="k-btn__loader"><KProgressCircular indeterminate :width="2" size="1.5em" /></span>
    <span v-if="prependIcon" class="k-btn__prepend"><KIcon :icon="prependIcon" /></span>
    <span class="k-btn__content">
      <KIcon v-if="typeof icon === 'string' && icon" :icon="icon" />
      <slot>{{ text }}</slot>
    </span>
    <span v-if="appendIcon" class="k-btn__prepend"><KIcon :icon="appendIcon" /></span>
  </button>
</template>
