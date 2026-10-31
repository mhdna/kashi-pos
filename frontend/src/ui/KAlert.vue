<script setup lang="ts">
import { computed } from "vue";
import KIcon from "./KIcon";
import { colorVars } from "./util";

const props = defineProps<{ type?: string; variant?: string; density?: string; prominent?: boolean; color?: string; title?: string; text?: string; icon?: string | boolean }>();
const icons: Record<string, string> = {
  error: "mdi-close-circle",
  info: "mdi-information",
  success: "mdi-check-circle",
  warning: "mdi-alert-circle",
};
const color = computed(() => props.color ?? props.type);
const variant = computed(() => props.variant ?? "flat");
const icon = computed(() => (typeof props.icon === "string" ? props.icon : props.icon === false ? undefined : props.type ? icons[props.type] : undefined));
</script>
<template>
  <div
    class="k-alert"
    :class="[color && 'k-alert--colored', `k-alert--variant-${variant}`, props.density && props.density !== 'default' && `k-alert--density-${props.density}`, prominent && 'k-alert--prominent']"
    :style="colorVars(color)"
    role="alert"
  >
    <div v-if="icon" class="k-alert__prepend"><KIcon :icon="icon" /></div>
    <div class="k-alert__content">
      <div v-if="title" class="k-alert-title">{{ title }}</div>
      <slot>{{ text }}</slot>
    </div>
  </div>
</template>
