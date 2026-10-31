<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { nextZ } from "./overlay";
import { placeBox } from "./position";
import { toUnit } from "./util";

const props = withDefaults(defineProps<{ text?: string; location?: string; openOnClick?: boolean; maxWidth?: string | number }>(), { location: "bottom" });
const open = ref(false);
const content = ref<HTMLElement | null>(null);
const zIndex = ref(2000);
const pos = ref({ left: 0, top: 0, ready: false });
let anchor: HTMLElement | null = null;

function show(event: Event) {
  anchor = event.currentTarget as HTMLElement;
  open.value = true;
}
function hide() {
  open.value = false;
}
const activatorProps = computed(() => ({
  onMouseenter: show,
  onMouseleave: hide,
  onFocus: show,
  onBlur: hide,
  ...(props.openOnClick ? { onClick: (e: Event) => (open.value ? hide() : show(e)) } : {}),
}));

watch(open, async (value) => {
  if (!value) {
    pos.value.ready = false;
    return;
  }
  zIndex.value = nextZ() + 10;
  await nextTick();
  const el = content.value;
  if (!anchor || !el) return;
  const { left, top } = placeBox(anchor.getBoundingClientRect(), el.offsetWidth, el.offsetHeight, { width: window.innerWidth, height: window.innerHeight }, props.location, 4);
  pos.value = { left, top, ready: true };
});
</script>

<template>
  <slot name="activator" :props="activatorProps" />
  <Teleport to="body">
    <div v-if="open" class="k-overlay k-tooltip" :style="{ zIndex }">
      <div ref="content" class="k-overlay__content" :style="{ left: `${pos.left}px`, top: `${pos.top}px`, maxWidth: toUnit(maxWidth), visibility: pos.ready ? 'visible' : 'hidden' }">
        <slot>{{ text }}</slot>
      </div>
    </div>
  </Teleport>
</template>
