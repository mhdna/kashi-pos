<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";
import { nextZ, useEscapeToClose } from "./overlay";
import { placeBox } from "./position";

const props = withDefaults(
  defineProps<{ modelValue?: boolean; location?: string; closeOnContentClick?: boolean; gap?: number; matchWidth?: boolean; maxHeight?: number | string; anchor?: HTMLElement | null }>(),
  { modelValue: false, location: "bottom start", closeOnContentClick: true, gap: 0 },
);
const emit = defineEmits<{ "update:modelValue": [value: boolean] }>();

const open = computed(() => props.modelValue);
const marker = ref<HTMLElement | null>(null);
const content = ref<HTMLElement | null>(null);
const zIndex = ref(2000);
const pos = ref({ left: 0, top: 0, minWidth: undefined as number | undefined, ready: false });
let activator: HTMLElement | null = null;

function setOpen(value: boolean) {
  emit("update:modelValue", value);
}
function onActivatorClick(event: MouseEvent) {
  activator = (event.currentTarget as HTMLElement | null) ?? activator;
  setOpen(!open.value);
}
const activatorProps = computed(() => ({ onClick: onActivatorClick, "aria-haspopup": "menu" as const, "aria-expanded": open.value }));

function anchorEl(): HTMLElement | null {
  return props.anchor ?? activator ?? ((marker.value?.previousElementSibling as HTMLElement | null) ?? marker.value?.parentElement ?? null);
}

function place() {
  const anchor = anchorEl();
  const el = content.value;
  if (!anchor || !el) return;
  const rect = anchor.getBoundingClientRect();
  const rtl = getComputedStyle(anchor).direction === "rtl";
  const width = props.matchWidth ? Math.max(el.offsetWidth, rect.width) : el.offsetWidth;
  const { left, top } = placeBox(rect, width, el.offsetHeight, { width: window.innerWidth, height: window.innerHeight }, props.location, props.gap, rtl);
  pos.value = { left, top, minWidth: props.matchWidth ? rect.width : undefined, ready: true };
}

watch(open, async (value) => {
  if (!value) {
    pos.value.ready = false;
    return;
  }
  zIndex.value = nextZ();
  await nextTick();
  place();
});

function onPointerDown(event: PointerEvent) {
  if (!open.value) return;
  const target = event.target as Node;
  if (content.value?.contains(target)) return;
  const anchor = anchorEl();
  if (anchor?.contains(target)) return;
  setOpen(false);
}
document.addEventListener("pointerdown", onPointerDown, true);
window.addEventListener("resize", place);
onBeforeUnmount(() => {
  document.removeEventListener("pointerdown", onPointerDown, true);
  window.removeEventListener("resize", place);
});
useEscapeToClose(open, () => setOpen(false));

function onContentClick() {
  if (props.closeOnContentClick) setOpen(false);
}
defineExpose({ place });
</script>

<template>
  <slot name="activator" :props="activatorProps" :is-active="open" />
  <span ref="marker" hidden />
  <Teleport to="body">
    <div v-if="open" class="k-overlay k-menu" :style="{ zIndex }">
      <div
        ref="content"
        class="k-overlay__content"
        :style="{ left: `${pos.left}px`, top: `${pos.top}px`, minWidth: pos.minWidth ? `${pos.minWidth}px` : undefined, maxHeight: maxHeight ? `${maxHeight}px` : undefined, visibility: pos.ready ? 'visible' : 'hidden' }"
        @click="onContentClick"
      >
        <slot />
      </div>
    </div>
  </Teleport>
</template>
