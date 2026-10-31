<script setup lang="ts">
import { computed, useAttrs } from "vue";
import KIcon from "./KIcon";
import { colorVars } from "./util";

const props = defineProps<{ variant?: string; size?: string; color?: string; label?: boolean; prependIcon?: string; appendIcon?: string; icon?: string; text?: string; density?: string; closable?: boolean }>();
const emit = defineEmits<{ "click:close": [event: MouseEvent] }>();
const attrs = useAttrs();
const variant = computed(() => props.variant ?? "tonal");
const startIcon = computed(() => props.prependIcon ?? props.icon);
</script>
<template>
  <span
    class="k-chip"
    :class="[
      `k-chip--variant-${variant}`,
      color && 'k-chip--colored',
      size && size !== 'default' && `k-chip--size-${size}`,
      label && 'k-chip--label',
      density && density !== 'default' && `k-chip--density-${density}`,
      attrs.onClick && 'k-chip--link',
    ]"
    :style="colorVars(color)"
  >
    <span class="k-chip__content">
      <KIcon v-if="startIcon" :icon="startIcon" start />
      <slot>{{ text }}</slot>
      <KIcon v-if="appendIcon" :icon="appendIcon" end />
      <KIcon v-if="closable" icon="mdi-close-circle" class="k-chip__close" @click.stop="emit('click:close', $event)" />
    </span>
  </span>
</template>
