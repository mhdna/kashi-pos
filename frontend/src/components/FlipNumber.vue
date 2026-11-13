<template>
  <span
    class="flip-number"
    :class="`flip-number--${direction}`"
    data-test="flip-number"
    role="img"
    :aria-label="String(value)"
    :style="{ '--n': digits.length }"
  >
    <!-- A row of split-flap cards, one per digit. Only the digits that change
         flip, like a flip clock: 477 to 478 turns the last card, 479 to 480
         turns two. Cards are keyed from the right, so the units card stays the
         same card when the number grows a digit or loses one. -->
    <!-- Everything that changes size together: the digits, the bar behind them and
         the ring over them grow as one while the count changes, then settle. -->
    <span class="flip-stage" :class="popClass" data-test="flip-stage">
      <!-- One tinted background behind the whole run of digits that changed, so
           several turning cards share a single highlight. A count only changes
           a run at the right end (a carry or a borrow), so one span covers it. -->
      <span
        v-if="tint"
        :key="run"
        class="flip-tint"
        data-test="flip-tint"
        :data-from="tint.from"
        :data-count="tint.count"
        :style="{ '--from': tint.from, '--count': tint.count }"
      />
      <!-- Its ring, drawn above the cards: behind them a neighbouring card would
           cover the ring's side and a lone turning digit would lose its edge. -->
      <span v-if="tint" :key="`ring${run}`" class="flip-ring" data-test="flip-ring" :style="{ '--from': tint.from, '--count': tint.count }" />
      <span class="flip-digits">
        <FlipDigit v-for="(char, index) in digits" :key="digits.length - index" :char="char" :direction="direction" />
      </span>
    </span>
  </span>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import FlipDigit from "./FlipDigit.vue";
import { changedRun } from "@/lib/flipDigits";

const props = defineProps<{ value: number }>();

const digits = computed(() => String(props.value).split(""));
// Which way the count went. Set before the digits update, so every card that
// turns knows it.
const direction = ref<"up" | "down">("up");
const tint = ref<{ from: number; count: number } | null>(null);
const run = ref(0);
// Two names for the same grow, swapped on every tap: a class that stays put does
// not restart its animation, so tapping fast would show nothing new.
const popClass = computed(() => (tint.value ? (run.value % 2 ? "flip-stage--odd" : "flip-stage--even") : ""));
let timer: ReturnType<typeof setTimeout> | undefined;

// How long the highlight lasts (the cards' own flips are shorter).
const HIGHLIGHT_MS = 620;

watch(
  () => props.value,
  (now, before) => {
    direction.value = now >= before ? "up" : "down";
    tint.value = changedRun(before, now);
    run.value += 1;
    clearTimeout(timer);
    timer = setTimeout(() => {
      tint.value = null;
    }, HIGHLIGHT_MS);
  },
);
</script>

<style scoped>
.flip-number {
  --pad: 3px;
  /* Cards touch: a digit is about 0.6em wide, so a 14px card fits it with no
     space around it. */
  --gap: 0px;
  /* However many digits there are, they share the width: three digits get
     wide cards, four or five get narrower ones, never overflowing. */
  --cell: min(14px, calc((50px - (var(--n) - 1) * var(--gap)) / var(--n)));
  --tint: #a86a0c;
  /* Adding one is the loud one (a big amber ring, the number growing a lot);
     taking one off is a quiet white flash and a small grow. */
  --flash: 255, 190, 40;
  --ring: 5px;
  --grow: 1.3;
  position: relative;
  box-sizing: border-box;
  display: block;
  width: 56px;
  height: 40px;
  padding: var(--pad);
  border-radius: 8px;
  background: #0b1320;
  box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.6), 0 1px 0 rgba(255, 255, 255, 0.08);
  font-size: min(1.4rem, calc(var(--cell) * 1.5));
  font-weight: 800;
  font-variant-numeric: tabular-nums;
  color: #fff;
}
/* Warm amber when the count goes up, slate blue when it goes down. */
.flip-number--down {
  --tint: #46608f;
  --flash: 255, 255, 255;
  --ring: 2px;
  --grow: 1.1;
}

/* The digits, the bar and the ring, as one piece that grows and settles. */
.flip-stage {
  position: relative;
  display: flex;
  justify-content: center;
  height: 100%;
}
.flip-stage--even {
  animation-name: flip-stage-pop !important;
}
.flip-stage--odd {
  animation-name: flip-stage-pop-b !important;
}
.flip-stage--even,
.flip-stage--odd {
  animation-duration: 0.6s !important;
  animation-delay: 0s !important;
  animation-timing-function: ease-out !important;
  animation-fill-mode: both !important;
}
.flip-digits {
  display: flex;
  align-items: stretch;
  gap: var(--gap);
  height: 100%;
}

/* Sits behind the cards, from the first changed card to the last one, and
   fades back to the card colour. The cards let it show through while they turn. */
.flip-tint,
.flip-ring {
  position: absolute;
  top: 0;
  bottom: 0;
  /* Centred like the cards: the row is as wide as its digits. */
  left: calc(50% - (var(--n) * var(--cell) + (var(--n) - 1) * var(--gap)) / 2 + var(--from) * (var(--cell) + var(--gap)));
  width: calc(var(--count) * var(--cell) + (var(--count) - 1) * var(--gap));
  border-radius: 4px;
  pointer-events: none;
  animation-delay: 0s !important;
  animation-fill-mode: both !important;
}
.flip-tint {
  z-index: 0;
  background: var(--tint);
  animation-name: flip-tint-fade !important;
  animation-duration: 0.6s !important;
  animation-timing-function: ease-out !important;
}
/* Its ring is above the cards, so a lone turning digit keeps its whole outline. */
.flip-ring {
  z-index: 3;
  animation-name: flip-ring-pop !important;
  animation-duration: 0.6s !important;
  animation-timing-function: ease-out !important;
}

@keyframes flip-stage-pop {
  0% { transform: scale(1); }
  25% { transform: scale(var(--grow)); }
  100% { transform: scale(1); }
}
@keyframes flip-stage-pop-b {
  0% { transform: scale(1); }
  25% { transform: scale(var(--grow)); }
  100% { transform: scale(1); }
}
@keyframes flip-ring-pop {
  0% { box-shadow: 0 0 0 0 rgba(var(--flash), 0.9); }
  25% { box-shadow: 0 0 0 var(--ring) rgba(var(--flash), 0.95); }
  100% { box-shadow: 0 0 0 0 rgba(var(--flash), 0); }
}
@keyframes flip-tint-fade {
  0% { background: var(--tint); }
  55% { background: var(--tint); }
  100% { background: #16243b; }
}
</style>
