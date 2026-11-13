<template>
  <span class="flip" :class="flipping && [`flip--${direction}`, run % 2 ? 'flip--odd' : 'flip--even']"
    :style="{ '--oxc': originCells }"
    data-test="flip-digit"
    aria-hidden="true"
  >
    <!-- One digit as a split-flap card, like a flip clock. The top half
         already shows the new digit and the bottom half the old one; the old
         top half then folds down over the middle and the new bottom half
         unfolds under it. -->
    <span class="flip-half flip-top"><span class="flip-num">{{ char }}</span></span>
    <span class="flip-half flip-bottom"><span class="flip-num">{{ flipping ? previous : char }}</span></span>
    <template v-if="flipping">
      <!-- The shadow the falling flap throws on the lower half. -->
      <span :key="`s${run}`" class="flip-shade" />
      <span :key="`t${run}`" class="flip-half flip-top flip-flap flip-flap-top">
        <span class="flip-num">{{ previous }}</span>
      </span>
      <span :key="`b${run}`" class="flip-half flip-bottom flip-flap flip-flap-bottom">
        <span class="flip-num">{{ char }}</span>
      </span>
    </template>
  </span>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";

// `direction` is the whole number's: which way the count went, not which way
// this one digit went (479 to 480 is "up" for the 9 turning into a 0).
const props = defineProps<{
  char: string;
  direction: "up" | "down";
  // Where this card grows from while it turns: how far (in cards) from its left
  // edge the middle of the whole turning run is, so a run grows as one piece.
  originCells?: number;
}>();

const previous = ref(props.char);
const flipping = ref(false);
const run = ref(0);
let timer: ReturnType<typeof setTimeout> | undefined;

// The longest part: the highlight (0.6s). The fold and unfold take 0.38s.
const FLIP_MS = 620;

watch(
  () => props.char,
  (_now, before) => {
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
  z-index: 1;
  display: block;
  flex: 0 0 var(--cell, 17px);
  width: var(--cell, 17px);
  height: 100%;
  perspective: 140px;
  /* Only the outer corners of the row are round; cards in between are square. */
  --rl: 0px;
  --rr: 0px;
  border-radius: var(--rl) var(--rr) var(--rr) var(--rl);
  --base-bg: #16243b;
  background: var(--base-bg);
  box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.6);
}
.flip:first-child {
  --rl: 4px;
}
.flip:last-child {
  --rr: 4px;
}
/* Touching cards: a thin line between them rather than a gap. */
.flip + .flip {
  border-left: 1px solid rgba(0, 0, 0, 0.45);
  box-sizing: border-box;
}
/* Two neighbours turning together are one highlighted square: no line
   between them while they do. */
.flip--up + .flip--up,
.flip--down + .flip--down {
  border-left-color: transparent;
}

.flip-half {
  position: absolute;
  left: 0;
  right: 0;
  height: 50%;
  overflow: hidden;
  isolation: isolate;
  backface-visibility: hidden;
}
.flip-top {
  top: 0;
  border-radius: var(--rl) var(--rr) 0 0;
  transform-origin: bottom;
}
.flip-bottom {
  bottom: 0;
  border-radius: 0 0 var(--rr) var(--rl);
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
  border-radius: 0 0 var(--rr) var(--rl);
  background: linear-gradient(180deg, rgba(0, 0, 0, 0.75), rgba(0, 0, 0, 0));
  pointer-events: none;
  animation-name: flip-shade !important;
  animation-duration: 0.38s !important;
  animation-delay: 0s !important;
  animation-timing-function: ease-in-out !important;
  animation-fill-mode: both !important;
}

/* The ring and the colour behind the digits belong to the number as a whole
   (FlipNumber's one .flip-tint bar and .flip-ring), not to each card: a turning
   card draws none of its own, so several turning together read as one square.
   It does grow while it turns, though, each card about the middle of the whole
   turning run (--oxc, in cards), so the run grows as one piece. A card that did
   not change stays as it is. */
.flip--up,
.flip--down {
  box-shadow: none;
  transform-origin: calc(var(--oxc, 0.5) * var(--cell, 17px)) 50%;
  animation-duration: 0.6s !important;
  animation-delay: 0s !important;
  animation-timing-function: ease-out !important;
  animation-fill-mode: both !important;
}
/* Two names for the same grow, swapped on every tap: a class that stays put does
   not restart its animation, so tapping fast would show nothing new. */
.flip--even {
  animation-name: flip-grow !important;
}
.flip--odd {
  animation-name: flip-grow-b !important;
}
/* The colour behind the digits while they turn is the number's one shared
   highlight (FlipNumber's .flip-tint, behind all the cards). So a turning card
   lets it show through instead of painting its own; only a flap, which has to
   cover what is under it as it turns, carries the same tint, fading with it. */
.flip-half::before {
  content: "";
  position: absolute;
  inset: 0;
  z-index: -1;
  background: var(--base-bg);
}
.flip--up,
.flip--down {
  background: transparent;
}
.flip--up .flip-half:not(.flip-flap)::before,
.flip--down .flip-half:not(.flip-flap)::before {
  background: transparent;
}
.flip-flap::before {
  background: var(--tint, #46608f);
  animation-name: flip-bg !important;
  animation-duration: 0.6s !important;
  animation-delay: 0s !important;
  animation-timing-function: ease-out !important;
  animation-fill-mode: both !important;
}

@keyframes flip-grow {
  0% { transform: scale(1); }
  25% { transform: scale(var(--grow, 1.2)); }
  100% { transform: scale(1); }
}
@keyframes flip-grow-b {
  0% { transform: scale(1); }
  25% { transform: scale(var(--grow, 1.2)); }
  100% { transform: scale(1); }
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
@keyframes flip-bg {
  0% { background: var(--tint); }
  55% { background: var(--tint); }
  100% { background: var(--base-bg); }
}
</style>
