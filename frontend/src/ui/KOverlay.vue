<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { nextZ, useEscapeToClose } from "./overlay";
import { resolveColor } from "./colors";

defineOptions({ inheritAttrs: false });
const props = withDefaults(defineProps<{ modelValue?: boolean; persistent?: boolean; scrim?: boolean | string; opacity?: number | string }>(), { modelValue: false, scrim: true });
const emit = defineEmits<{ "update:modelValue": [value: boolean] }>();
const open = computed(() => props.modelValue);
const zIndex = ref(2000);
watch(open, (v) => v && (zIndex.value = nextZ()), { immediate: true });
useEscapeToClose(open, () => emit("update:modelValue", false), () => !props.persistent);
const scrimStyle = computed(() => ({
  background: typeof props.scrim === "string" ? resolveColor(props.scrim === "background" ? "background" : props.scrim) : undefined,
  opacity: props.opacity,
}));
</script>
<template>
  <Teleport to="body">
    <div v-if="open" v-bind="$attrs" class="k-overlay" :style="{ zIndex }">
      <div v-if="scrim !== false" class="k-overlay__scrim" :style="scrimStyle" />
      <div class="k-overlay__content"><slot /></div>
    </div>
  </Teleport>
</template>
