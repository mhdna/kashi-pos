<template>
  <k-dialog v-model="open" fullscreen :scrim="false" transition="fade-transition">
    <div
      ref="stage"
      class="photo-viewer"
      @pointerdown="onPointerDown"
      @pointermove="onPointerMove"
      @pointerup="onPointerUp"
      @pointercancel="onPointerUp"
      @wheel.prevent="onWheel"
      @dblclick="toggleZoom"
    >
      <img
        v-if="current"
        :src="current"
        alt="Photo"
        class="photo-viewer-img"
        draggable="false"
        :style="{ transform: `translate(${x}px, ${y}px) scale(${scale})` }"
      />
      <k-btn class="photo-viewer-close" icon="mdi-close" variant="flat" aria-label="Close" @pointerdown.stop @click="open = false" />
      <template v-if="photos.length > 1">
        <k-btn class="photo-viewer-nav photo-viewer-nav--prev" icon="mdi-chevron-left" variant="flat" aria-label="Previous photo" @pointerdown.stop @click="step(-1)" />
        <k-btn class="photo-viewer-nav photo-viewer-nav--next" icon="mdi-chevron-right" variant="flat" aria-label="Next photo" @pointerdown.stop @click="step(1)" />
        <div class="photo-viewer-count">{{ index + 1 }} / {{ photos.length }}</div>
      </template>
    </div>
  </k-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";

// Full-screen photo with pinch-to-zoom (two fingers), mouse-wheel zoom, drag
// to pan and double-tap to zoom in / reset. Opened from the photo thumbnails
// in the expense, loan and transfer details.

const props = defineProps<{ photos: string[] }>();
const open = defineModel<boolean>({ default: false });
const index = defineModel<number>("index", { default: 0 });

const MIN = 1;
const MAX = 6;
const scale = ref(1);
const x = ref(0);
const y = ref(0);
const stage = ref<HTMLElement | null>(null);
const current = computed(() => props.photos[index.value] ?? "");

function reset() {
  scale.value = 1;
  x.value = 0;
  y.value = 0;
}
watch([open, index], reset);

function step(delta: number) {
  const n = props.photos.length;
  index.value = (index.value + delta + n) % n;
}

// Zooms to `next`, keeping the point under (cx, cy) -- screen coordinates --
// where it is.
function zoomAt(next: number, cx: number, cy: number) {
  next = Math.min(MAX, Math.max(MIN, next));
  const rect = stage.value?.getBoundingClientRect();
  const ox = cx - (rect ? rect.left + rect.width / 2 : 0);
  const oy = cy - (rect ? rect.top + rect.height / 2 : 0);
  const ratio = next / scale.value;
  x.value = ox - (ox - x.value) * ratio;
  y.value = oy - (oy - y.value) * ratio;
  scale.value = next;
  if (next === MIN) reset();
}

function onWheel(event: WheelEvent) {
  zoomAt(scale.value * (event.deltaY < 0 ? 1.15 : 1 / 1.15), event.clientX, event.clientY);
}

function toggleZoom(event: MouseEvent) {
  if (scale.value > 1) reset();
  else zoomAt(2.5, event.clientX, event.clientY);
}

// Active pointers: one drags, two pinch.
const pointers = new Map<number, { x: number; y: number }>();
let pinchStart = 0;
let pinchScale = 1;

function distance() {
  const [a, b] = [...pointers.values()];
  return a && b ? Math.hypot(a.x - b.x, a.y - b.y) : 0;
}

function onPointerDown(event: PointerEvent) {
  (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
  pointers.set(event.pointerId, { x: event.clientX, y: event.clientY });
  if (pointers.size === 2) {
    pinchStart = distance();
    pinchScale = scale.value;
  }
}

function onPointerMove(event: PointerEvent) {
  const last = pointers.get(event.pointerId);
  if (!last) return;
  const now = { x: event.clientX, y: event.clientY };
  pointers.set(event.pointerId, now);
  if (pointers.size === 2 && pinchStart > 0) {
    const [a, b] = [...pointers.values()];
    zoomAt((pinchScale * distance()) / pinchStart, (a!.x + b!.x) / 2, (a!.y + b!.y) / 2);
  } else if (pointers.size === 1 && scale.value > 1) {
    x.value += now.x - last.x;
    y.value += now.y - last.y;
  }
}

function onPointerUp(event: PointerEvent) {
  pointers.delete(event.pointerId);
  if (pointers.size < 2) pinchStart = 0;
}
</script>

<style scoped>
.photo-viewer {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.92);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  touch-action: none;
  user-select: none;
}

.photo-viewer-img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  transform-origin: center center;
  cursor: zoom-in;
}

.photo-viewer-close {
  position: absolute;
  top: 16px;
  right: 16px;
}

.photo-viewer-nav {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
}

.photo-viewer-nav--prev {
  left: 16px;
}

.photo-viewer-nav--next {
  right: 16px;
}

.photo-viewer-count {
  position: absolute;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%);
  color: #fff;
  font-weight: 600;
}
</style>
