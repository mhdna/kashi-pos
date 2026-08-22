<template>
  <div class="calculator-keypad">
    <div class="keypad-numbers">
      <v-btn
        v-for="n in digitKeys"
        :key="n"
        class="keypad-button keypad-number text-h4"
        variant="text"
        @click="$emit('digit', n)"
        >{{ n }}</v-btn
      >
      <v-btn
        class="keypad-button keypad-number text-h4"
        variant="text"
        @click="$emit('digit', '.')"
        >.</v-btn
      >
      <v-btn class="keypad-button keypad-number text-h4" variant="text" @click="$emit('digit', '0')">0</v-btn>
      <v-btn class="keypad-button keypad-number text-h4" variant="text" @click="$emit('digit', '00')">00</v-btn>
    </div>
  </div>
</template>

<script setup lang="ts">
defineEmits<{ digit: [value: string] }>();
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
  grid-template-columns: repeat(3, minmax(0, 1fr));
  grid-template-rows: repeat(4, minmax(0, 1fr));
  width: 100%;
  height: 100%;
  gap: 10px;
}
.keypad-button {
  height: 100%;
  width: 100%;
  border-radius: 4px;
  border: 1px solid rgba(var(--v-border-color), 0.18);
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

@container (max-width: 420px) {
  .keypad-numbers { gap: 6px; }
}
</style>
