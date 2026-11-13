<template>
  <span class="flip-number" data-test="flip-number" role="img" :aria-label="String(value)" :style="{ '--n': digits.length }">
    <!-- A row of split-flap cards, one per digit. Only the digits that change
         flip, like a flip clock: 477 to 478 turns the last card, 479 to 480
         turns two. Cards are keyed from the right, so the units card stays the
         same card when the number grows a digit or loses one. -->
    <FlipDigit v-for="(char, index) in digits" :key="digits.length - index" :char="char" :direction="direction" />
  </span>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import FlipDigit from "./FlipDigit.vue";

const props = defineProps<{ value: number }>();

const digits = computed(() => String(props.value).split(""));
// Which way the count went. Set before the digits update, so every card that
// turns knows it.
const direction = ref<"up" | "down">("up");
watch(
  () => props.value,
  (now, before) => {
    direction.value = now >= before ? "up" : "down";
  },
);
</script>

<style scoped>
.flip-number {
  box-sizing: border-box;
  display: flex;
  align-items: stretch;
  justify-content: center;
  gap: 2px;
  width: 56px;
  height: 40px;
  padding: 3px;
  border-radius: 8px;
  background: #0b1320;
  box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.6), 0 1px 0 rgba(255, 255, 255, 0.08);
  /* However many digits there are, they share the width: three digits get
     wide cards, four or five get narrower ones, never overflowing. */
  --cell: min(18px, calc((50px - (var(--n) - 1) * 2px) / var(--n)));
  font-size: min(1.4rem, calc(var(--cell) * 1.3));
  font-weight: 800;
  font-variant-numeric: tabular-nums;
  color: #fff;
}
</style>
