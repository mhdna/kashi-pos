<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { nextZ, useEscapeToClose } from "./overlay";
import { toUnit } from "./util";

defineOptions({ inheritAttrs: false });
const props = withDefaults(
  defineProps<{
    modelValue?: boolean;
    maxWidth?: string | number;
    width?: string | number;
    height?: string | number;
    persistent?: boolean;
    scrollable?: boolean;
    fullscreen?: boolean;
    scrim?: boolean | string;
  }>(),
  { modelValue: false, scrim: true },
);
const emit = defineEmits<{ "update:modelValue": [value: boolean]; afterEnter: []; afterLeave: [] }>();

const open = computed(() => props.modelValue);
const zIndex = ref(2000);
function close() {
  emit("update:modelValue", false);
}
useEscapeToClose(open, close, () => !props.persistent);

watch(
  open,
  (value, old) => {
    if (value) {
      zIndex.value = nextZ();
      void nextTick(() => emit("afterEnter"));
    } else if (old) {
      void nextTick(() => emit("afterLeave"));
    }
  },
  { immediate: false },
);

// A click that starts and ends outside the content closes the dialog.
let downOutside = false;
function onDown(event: MouseEvent) {
  downOutside = !(event.target as HTMLElement).closest(".k-overlay__content");
}
function onUp(event: MouseEvent) {
  if (downOutside && !props.persistent && !(event.target as HTMLElement).closest(".k-overlay__content")) close();
  downOutside = false;
}
const contentStyle = computed(() => ({ maxWidth: toUnit(props.maxWidth), width: toUnit(props.width), height: toUnit(props.height) }));
</script>

<template>
  <slot name="activator" :props="{ onClick: () => emit('update:modelValue', true) }" />
  <Teleport to="body">
    <div
      v-if="open"
      v-bind="$attrs"
      class="k-overlay k-dialog"
      :class="[scrollable && 'k-dialog--scrollable', fullscreen && 'k-dialog--fullscreen']"
      :style="{ zIndex }"
      @mousedown="onDown"
      @mouseup="onUp"
    >
      <div v-if="scrim !== false" class="k-overlay__scrim" />
      <div class="k-overlay__content" :style="contentStyle" role="dialog" aria-modal="true">
        <slot :is-active="{ value: open }" />
      </div>
    </div>
  </Teleport>
</template>
