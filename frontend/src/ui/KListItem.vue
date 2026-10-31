<script setup lang="ts">
import { computed, inject, useAttrs } from "vue";
import KIcon from "./KIcon";
import { listKey } from "./keys";
import { colorVars, roundedStyle } from "./util";

const props = defineProps<{
  title?: string | number;
  subtitle?: string | number;
  prependIcon?: string;
  appendIcon?: string;
  active?: boolean;
  activeColor?: string;
  baseColor?: string;
  link?: boolean;
  rounded?: string | boolean;
  density?: string;
  value?: unknown;
  disabled?: boolean;
}>();
const slots = defineSlots<{ default?(): unknown; title?(): unknown; subtitle?(): unknown; prepend?(): unknown; append?(): unknown }>();
const list = inject(listKey, { nav: false, density: "default" });
const attrs = useAttrs();
const density = computed(() => props.density ?? list.density);
const clickable = computed(() => props.link || !!attrs.onClick);
const color = computed(() => (props.active ? props.activeColor : undefined) ?? props.baseColor);
const style = computed(() => {
  const s: Record<string, string | undefined> = { ...colorVars(color.value) };
  const r = roundedStyle(props.rounded);
  if (r) s.borderRadius = r;
  return s;
});
const twoLine = computed(() => props.subtitle !== undefined || !!slots.subtitle);
</script>
<template>
  <div
    class="k-list-item"
    :class="[
      clickable && 'k-list-item--link',
      active && 'k-list-item--active',
      color && 'k-list-item--colored',
      disabled && 'k-list-item--disabled',
      twoLine && 'k-list-item--two-line',
      density !== 'default' && `k-list-item--density-${density}`,
      rounded !== undefined && rounded !== false && 'k-list-item--rounded',
    ]"
    :style="style"
    :role="clickable ? 'button' : 'listitem'"
    :tabindex="clickable && !disabled ? 0 : undefined"
    @keydown.enter.self="($event.target as HTMLElement).click()"
  >
    <div v-if="prependIcon || $slots.prepend" class="k-list-item__prepend">
      <slot name="prepend"><KIcon :icon="prependIcon" /></slot>
    </div>
    <div class="k-list-item__content">
      <div v-if="title !== undefined || $slots.title" class="k-list-item-title"><slot name="title">{{ title }}</slot></div>
      <div v-if="subtitle !== undefined || $slots.subtitle" class="k-list-item-subtitle"><slot name="subtitle">{{ subtitle }}</slot></div>
      <slot />
    </div>
    <div v-if="appendIcon || $slots.append" class="k-list-item__append">
      <slot name="append"><KIcon :icon="appendIcon" /></slot>
    </div>
  </div>
</template>
