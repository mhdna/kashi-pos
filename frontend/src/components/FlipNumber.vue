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
    <span class="flip-digits">
      <FlipDigit v-for="(char, index) in digits" :key="digits.length - index" :char="char" :direction="direction" />
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
  position: relative;
  box-sizing: border-box;
  display: flex;
  align-items: stretch;
  justify-content: center;
  gap: var(--gap);
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
.flip-digits {
  display: flex;
  align-items: stretch;
  gap: var(--gap);
  height: 100%;
}
/* Warm amber when the count goes up, slate blue when it goes down. Adding one is
   the loud one (a big amber ring); taking one off a quiet white flash. */
.flip-number {
  --flash: 255, 190, 40;
  --ring: 5px;
  --grow: 1.1;
}
.flip-number--down {
  --tint: #46608f;
  --flash: 255, 255, 255;
  --ring: 2px;
  --grow: 1.04;
}

/* Sits behind the cards, from the first changed card to the last one, and
   fades back to the card colour. The cards let it show through while they turn. */
.flip-tint {
  position: absolute;
  top: var(--pad);
  bottom: var(--pad);
  /* Centred like the cards: the row is as wide as its digits. */
  left: calc(50% - (var(--n) * var(--cell) + (var(--n) - 1) * var(--gap)) / 2 + var(--from) * (var(--cell) + var(--gap)));
  width: calc(var(--count) * var(--cell) + (var(--count) - 1) * var(--gap));
  border-radius: 4px;
  background: var(--tint);
  pointer-events: none;
  /* The one square: it fades, rings and pops as a single piece. */
  animation-name: flip-tint-fade, flip-tint-pop !important;
  animation-duration: 0.6s, 0.6s !important;
  animation-delay: 0s !important;
  animation-timing-function: ease-out !important;
  animation-fill-mode: both !important;
}

@keyframes flip-tint-pop {
  0% { transform: scale(1); box-shadow: 0 0 0 0 rgba(var(--flash), 0.9); }
  25% { transform: scale(var(--grow)); box-shadow: 0 0 0 var(--ring) rgba(var(--flash), 0.95); }
  100% { transform: scale(1); box-shadow: 0 0 0 0 rgba(var(--flash), 0); }
}
@keyframes flip-tint-fade {
  0% { background: var(--tint); }
  55% { background: var(--tint); }
  100% { background: #16243b; }
}
</style>
