<template>
  <div class="calculator-keypad">
    <div class="keypad-numbers">
      <k-btn
        v-for="n in digitKeys"
        :key="n"
        class="keypad-button keypad-number text-h4"
        variant="text"
        @click="$emit('digit', n)"
        >{{ n }}</k-btn
      >
      <k-btn
        class="keypad-button keypad-number text-h4"
        variant="text"
        @click="$emit('digit', '.')"
        >.</k-btn
      >
      <k-btn class="keypad-button keypad-number text-h4" variant="text" @click="$emit('digit', '0')">0</k-btn>
      <k-btn class="keypad-button keypad-number text-h4" variant="text" @click="$emit('digit', '00')">00</k-btn>
      <!-- + and - choose the next amount's direction by hand: + counts it as
           given by the customer, - as change handed back, whatever is owed. -->
      <k-btn
        class="keypad-button keypad-sign keypad-sign--plus text-h4"
        :class="{ 'keypad-sign--on': sign === '+' }"
        variant="text"
        aria-label="Add the next amount as given"
        :aria-pressed="sign === '+'"
        data-test="keypad-plus"
        @click="$emit('sign', '+')"
        >+</k-btn
      >
      <k-btn
        class="keypad-button keypad-sign keypad-sign--minus text-h4"
        :class="{ 'keypad-sign--on': sign === '-' }"
        variant="text"
        aria-label="Take the next amount off as change"
        :aria-pressed="sign === '-'"
        data-test="keypad-minus"
        @click="$emit('sign', '-')"
        >&minus;</k-btn
      >
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ sign?: "+" | "-" | null }>();
defineEmits<{ digit: [value: string]; sign: [value: "+" | "-"] }>();
const digitKeys = [1, 2, 3, 4, 5, 6, 7, 8, 9].map(String);
</script>

<style scoped>
.calculator-keypad {
  container-type: inline-size;
  height: 100%;
  display: flex;
}
.keypad-numbers {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr)) minmax(0, 0.8fr);
  grid-template-rows: repeat(4, minmax(0, 1fr));
  width: 100%;
  height: 100%;
  gap: 10px;
}
.keypad-button {
  height: 100%;
  width: 100%;
  border-radius: 4px;
  border: 1px solid rgba(var(--k-border-color), 0.18);
  font-variant-numeric: tabular-nums;
  font-size: 2rem !important;
  transition:
    border-color 120ms ease,
    background-color 120ms ease;
}
.keypad-button:hover {
  border-color: currentColor;
}
.keypad-number {
  min-width: 0;
  padding-inline: 6px;
}
.keypad-sign {
  grid-column: 4;
  grid-row: span 2;
  min-width: 0;
}
.keypad-sign--plus {
  grid-row: 1 / span 2;
}
.keypad-sign--minus {
  grid-row: 3 / span 2;
}
.keypad-sign--on {
  background: rgba(var(--k-theme-on-surface), 0.12);
  border-color: currentColor;
}

@container (max-width: 420px) {
  .keypad-numbers { gap: 6px; }
}
</style>
