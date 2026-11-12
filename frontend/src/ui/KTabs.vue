<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, provide, ref, watch } from "vue";
import { tabsKey } from "./keys";
import KIcon from "./KIcon";
import { arrowState, pageTarget, revealTarget, wheelToX } from "./tabsScroll";

const props = defineProps<{ modelValue?: unknown; color?: string; grow?: boolean; density?: string }>();
const emit = defineEmits<{ "update:modelValue": [value: unknown] }>();
provide(tabsKey, { current: () => props.modelValue, select: (v) => emit("update:modelValue", v), color: props.color });

// Tabs that don't fit scroll sideways: swipe, mouse wheel, or the edge arrows,
// and the chosen tab is brought into view.
const track = ref<HTMLElement | null>(null);
const prev = ref(false);
const next = ref(false);

function update() {
  const el = track.value;
  if (!el) return;
  const s = arrowState(el.scrollLeft, el.clientWidth, el.scrollWidth);
  prev.value = s.prev;
  next.value = s.next;
}

function scrollTo(left: number) {
  const el = track.value;
  if (!el) return;
  if (typeof el.scrollTo === "function") el.scrollTo({ left });
  else el.scrollLeft = left;
  update();
}

function page(dir: -1 | 1) {
  const el = track.value;
  if (el) scrollTo(pageTarget(el.scrollLeft, el.clientWidth, el.scrollWidth, dir));
}

function onWheel(e: WheelEvent) {
  const el = track.value;
  if (!el || el.scrollWidth <= el.clientWidth) return;
  e.preventDefault();
  el.scrollLeft += wheelToX(e.deltaX, e.deltaY);
  update();
}

function reveal() {
  const el = track.value;
  const tab = el?.querySelector<HTMLElement>(".k-tab--selected");
  if (!el || !tab) return;
  const target = revealTarget(el.scrollLeft, el.clientWidth, el.scrollWidth, tab.offsetLeft, tab.offsetWidth);
  if (target !== el.scrollLeft) scrollTo(target);
}

let observer: ResizeObserver | null = null;
onMounted(() => {
  update();
  reveal();
  if (typeof ResizeObserver !== "undefined" && track.value) {
    observer = new ResizeObserver(update);
    observer.observe(track.value);
  }
});
onBeforeUnmount(() => observer?.disconnect());
watch(() => props.modelValue, () => nextTick(reveal));
defineExpose({ update, page });
</script>
<template>
  <div class="k-tabs" :class="[grow && 'k-tabs--grow', density && density !== 'default' && `k-tabs--density-${density}`]">
    <button v-if="prev" type="button" class="k-tabs__arrow k-tabs__arrow--prev" aria-label="Earlier tabs" data-test="tabs-prev" @click="page(-1)">
      <KIcon icon="mdi-chevron-left" />
    </button>
    <div ref="track" class="k-tabs__track" role="tablist" @scroll.passive="update" @wheel="onWheel"><slot /></div>
    <button v-if="next" type="button" class="k-tabs__arrow k-tabs__arrow--next" aria-label="More tabs" data-test="tabs-next" @click="page(1)">
      <KIcon icon="mdi-chevron-right" />
    </button>
  </div>
</template>
