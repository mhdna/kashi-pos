<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from "vue";
import { colorVars } from "./util";

defineOptions({ inheritAttrs: false });
const props = withDefaults(
  defineProps<{ modelValue?: boolean; timeout?: number | string; location?: string; color?: string; variant?: string; multiLine?: boolean }>(),
  { modelValue: false, timeout: 5000, location: "bottom" },
);
const emit = defineEmits<{ "update:modelValue": [value: boolean] }>();

// All snackbars at one edge share a host element so they stack.
function hostFor(side: "top" | "bottom"): HTMLElement {
  const id = `k-snackbar-${side}`;
  let el = document.getElementById(id);
  if (!el) {
    el = document.createElement("div");
    el.id = id;
    el.className = `k-snackbar-host k-snackbar-host--${side}`;
    document.body.appendChild(el);
  }
  return el;
}
const host = computed(() => hostFor(props.location.startsWith("top") ? "top" : "bottom"));

let timer: ReturnType<typeof setTimeout> | undefined;
watch(
  () => [props.modelValue, props.timeout] as const,
  ([open, timeout]) => {
    clearTimeout(timer);
    const ms = Number(timeout);
    if (open && ms > 0) timer = setTimeout(() => emit("update:modelValue", false), ms);
  },
  { immediate: true },
);
onBeforeUnmount(() => clearTimeout(timer));
</script>

<template>
  <Teleport :to="host">
    <div v-if="modelValue" v-bind="$attrs" class="k-snackbar" role="status">
      <div class="k-snackbar__wrapper" :class="{ 'k-snackbar__wrapper--colored': color }" :style="colorVars(color)">
        <div class="k-snackbar__content"><slot /></div>
        <div v-if="$slots.actions" class="k-snackbar__actions"><slot name="actions" /></div>
      </div>
    </div>
  </Teleport>
</template>
