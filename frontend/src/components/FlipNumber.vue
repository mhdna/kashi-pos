<template>
  <span class="flip" :class="flipping && [`flip--${direction}`, run % 2 ? 'flip--odd' : 'flip--even']" data-test="flip-number" :aria-label="String(value)" role="img">
    <!-- A split-flap card, like a flip clock. The top half already shows the
         new number and the bottom half the old one; the old top half then
         folds down over the middle and the new bottom half unfolds under it. -->
    <span class="flip-half flip-top"><span class="flip-num">{{ value }}</span></span>
    <span class="flip-half flip-bottom"><span class="flip-num">{{ flipping ? previous : value }}</span></span>
    <template v-if="flipping">
      <!-- The shadow the falling flap throws on the lower half. -->
      <span :key="`s${run}`" class="flip-shade" />
      <!-- The colour wash over the card; keyed so every tap starts it afresh. -->
      <span :key="`w${run}`" class="flip-wash" />
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
const direction = ref<"up" | "down">("up");
const run = ref(0);
let timer: ReturnType<typeof setTimeout> | undefined;

// The longest part: the highlight (0.6s). The fold and unfold take 0.38s.
const FLIP_MS = 620;

watch(
  () => props.value,
  (now, before) => {
    previous.value = before;
    direction.value = now >= before ? "up" : "down";
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
  background: #16243b;
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
  /* Just enough to read as a hinge, not a cut through the digits. */
  background: rgba(0, 0, 0, 0.22);
  z-index: 5;
  pointer-events: none;
}

.flip-flap {
  z-index: 3;
}
/* All of these always play, whatever the animations setting says: it is
   feedback for a tap, not decoration. styles/settings.scss puts
   `animation: none !important` on every element, so each one sets its whole
   animation with !important too (an !important rule on a real selector beats
   the one on `*`). */
.flip-flap-top {
  animation-name: flip-fold !important;
  animation-duration: 0.14s !important;
  animation-delay: 0s !important;
  animation-timing-function: ease-in !important;
  animation-fill-mode: both !important;
}
/* Unfolds a little past flat and settles back: a flap landing. */
.flip-flap-bottom {
  animation-name: flip-unfold !important;
  animation-duration: 0.24s !important;
  animation-delay: 0.14s !important;
  animation-timing-function: cubic-bezier(0.3, 1.5, 0.5, 1) !important;
  animation-fill-mode: both !important;
}

.flip-shade {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 50%;
  z-index: 2;
  border-radius: 0 0 8px 8px;
  background: linear-gradient(180deg, rgba(0, 0, 0, 0.75), rgba(0, 0, 0, 0));
  pointer-events: none;
  animation-name: flip-shade !important;
  animation-duration: 0.38s !important;
  animation-delay: 0s !important;
  animation-timing-function: ease-in-out !important;
  animation-fill-mode: both !important;
}

/* The whole card pops, rings and washes over when the count changes. Adding
   one is the loud one (green, big ring, strong wash); taking one off is a
   quiet white flash, so the two never look alike. */
.flip--up {
  --flash: 67, 214, 110;
  --ring: 6px;
  --wash: 0.6;
  --grow: 1.14;
}
.flip--down {
  --flash: 255, 255, 255;
  --ring: 2px;
  --wash: 0.2;
  --grow: 1.05;
}
/* Two names for the same pop, swapped on every tap: a class that stays put
   does not restart its animation, so tapping fast would show nothing new. */
.flip--even {
  animation-name: flip-pop !important;
}
.flip--odd {
  animation-name: flip-pop-b !important;
}
.flip--up,
.flip--down {
  animation-duration: 0.6s !important;
  animation-delay: 0s !important;
  animation-timing-function: ease-out !important;
  animation-fill-mode: both !important;
}
.flip-wash {
  position: absolute;
  inset: 0;
  z-index: 6;
  border-radius: 8px;
  background: linear-gradient(180deg, rgba(var(--flash), 1), rgba(var(--flash), 0.45));
  pointer-events: none;
  animation-name: flip-wash !important;
  animation-duration: 0.6s !important;
  animation-delay: 0s !important;
  animation-timing-function: ease-out !important;
  animation-fill-mode: both !important;
}

@keyframes flip-fold {
  from { transform: rotateX(0); filter: brightness(1); }
  to { transform: rotateX(-90deg); filter: brightness(0.45); }
}
@keyframes flip-unfold {
  from { transform: rotateX(90deg); filter: brightness(0.45); }
  to { transform: rotateX(0); filter: brightness(1); }
}
@keyframes flip-shade {
  0% { opacity: 0; }
  35% { opacity: 1; }
  100% { opacity: 0; }
}
@keyframes flip-pop {
  0% { transform: scale(1); box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.6), 0 0 0 0 rgba(var(--flash), 0.9); }
  25% { transform: scale(var(--grow)); box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.6), 0 0 0 var(--ring) rgba(var(--flash), 0.95); }
  100% { transform: scale(1); box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.6), 0 0 0 0 rgba(var(--flash), 0); }
}
@keyframes flip-pop-b {
  0% { transform: scale(1); box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.6), 0 0 0 0 rgba(var(--flash), 0.9); }
  25% { transform: scale(var(--grow)); box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.6), 0 0 0 var(--ring) rgba(var(--flash), 0.95); }
  100% { transform: scale(1); box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.6), 0 0 0 0 rgba(var(--flash), 0); }
}
@keyframes flip-wash {
  0% { opacity: 0; }
  20% { opacity: var(--wash); }
  100% { opacity: 0; }
}
</style>
