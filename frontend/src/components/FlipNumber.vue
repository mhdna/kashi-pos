<template>
  <span class="flip" data-test="flip-number" :aria-label="String(value)" role="img">
    <!-- A split-flap card, like a flip clock. The top half already shows the
         new number and the bottom half the old one; the old top half then
         folds down over the middle and the new bottom half unfolds under it. -->
    <span class="flip-half flip-top"><span class="flip-num">{{ value }}</span></span>
    <span class="flip-half flip-bottom"><span class="flip-num">{{ flipping ? previous : value }}</span></span>
    <template v-if="flipping">
      <span :key="`t${run}`" class="flip-half flip-top flip-flap flip-flap-top">
        <span class="flip-num">{{ previous }}</span>
      </span>
      <span :key="`b${run}`" class="flip-half flip-bottom flip-flap flip-flap-bottom">
        <span class="flip-num">{{ value }}</span>
      </span>
    </template>
  </span>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";

const props = defineProps<{ value: number }>();

const previous = ref(props.value);
const flipping = ref(false);
const run = ref(0);
let timer: ReturnType<typeof setTimeout> | undefined;

// Total flip: the fold down (0.16s) plus the unfold (0.16s).
const FLIP_MS = 340;

watch(
  () => props.value,
  (now, before) => {
    previous.value = before;
    run.value += 1;
    flipping.value = true;
    clearTimeout(timer);
    timer = setTimeout(() => {
      flipping.value = false;
    }, FLIP_MS);
  },
);
</script>

<style scoped>
.flip {
  position: relative;
  display: block;
  width: 48px;
  height: 40px;
  perspective: 140px;
  border-radius: 8px;
  background: #0b1320;
  box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.6), 0 1px 0 rgba(255, 255, 255, 0.08);
  font-size: 1.4rem;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
  color: #fff;
}

.flip-half {
  position: absolute;
  left: 0;
  right: 0;
  height: 50%;
  overflow: hidden;
  background: linear-gradient(180deg, #1b2a42, #15233a);
  backface-visibility: hidden;
}
.flip-top {
  top: 0;
  border-radius: 8px 8px 0 0;
  transform-origin: bottom;
}
.flip-bottom {
  bottom: 0;
  border-radius: 0 0 8px 8px;
  transform-origin: top;
  background: linear-gradient(180deg, #121f34, #0f1b2d);
}

/* The number is as tall as the whole card; each half shows its own part. */
.flip-num {
  position: absolute;
  left: 0;
  right: 0;
  height: 200%;
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
}
.flip-top .flip-num { top: 0; }
.flip-bottom .flip-num { bottom: 0; }

/* The hinge line across the middle. */
.flip::after {
  content: "";
  position: absolute;
  left: 0;
  right: 0;
  top: 50%;
  height: 1px;
  background: rgba(0, 0, 0, 0.7);
  z-index: 5;
  pointer-events: none;
}

.flip-flap {
  z-index: 3;
}
/* Always plays, whatever the animations setting says: it is feedback for a
   tap, not decoration. styles/settings.scss puts `animation: none
   !important` on every element, so these set the whole animation with
   !important too (an !important rule on a real selector beats the one on
   `*`). */
.flip-flap-top {
  animation-name: flip-fold !important;
  animation-duration: 0.16s !important;
  animation-delay: 0s !important;
  animation-timing-function: ease-in !important;
  animation-fill-mode: both !important;
}
.flip-flap-bottom {
  animation-name: flip-unfold !important;
  animation-duration: 0.16s !important;
  animation-delay: 0.16s !important;
  animation-timing-function: ease-out !important;
  animation-fill-mode: both !important;
}

@keyframes flip-fold {
  from { transform: rotateX(0); filter: brightness(1); }
  to { transform: rotateX(-90deg); filter: brightness(0.5); }
}
@keyframes flip-unfold {
  from { transform: rotateX(90deg); filter: brightness(0.5); }
  to { transform: rotateX(0); filter: brightness(1); }
}
</style>
