<template>
  <div class="calculator-keypad">
    <div class="keypad-numbers">
      <v-btn
        v-for="n in digitKeys"
        :key="n"
        class="keypad-button keypad-number text-h6"
        @click="$emit('digit', n)"
        >{{ n }}</v-btn
      >
      <v-btn
        class="keypad-button keypad-number text-h6"
        variant="tonal"
        @click="$emit('digit', '.')"
        >.</v-btn
      >
      <v-btn class="keypad-button keypad-number text-h6" @click="$emit('digit', '0')">0</v-btn>
      <v-btn class="keypad-button keypad-number text-h6" @click="$emit('digit', '000')">000</v-btn>
    </div>
    <div class="keypad-actions">
      <v-btn
        class="keypad-button keypad-action text-h6"
        variant="tonal"
        aria-label="Backspace"
        @click="$emit('backspace')"
        >⌫</v-btn
      >
      <v-btn
        class="keypad-button keypad-action ca-button text-h6"
        variant="tonal"
        @click="$emit('clear')"
        >CA</v-btn
      >
    </div>
  </div>
</template>

<script setup lang="ts">
defineEmits<{ digit: [value: string]; backspace: []; clear: [] }>();
const digitKeys = [1, 2, 3, 4, 5, 6, 7, 8, 9].map(String);
</script>

<style scoped>
.calculator-keypad {
  container-type: inline-size;
  display: grid;
  grid-template-columns: minmax(0, 3fr) minmax(0, 1fr);
  column-gap: 18px;
}
.keypad-numbers {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  width: 100%;
  max-width: 216px;
  justify-self: end;
  gap: 8px;
}
.keypad-actions {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.keypad-button {
  min-height: 44px;
  border-radius: 12px;
  font-variant-numeric: tabular-nums;
  font-size: 1.25rem !important;
}
.keypad-number {
  min-height: 40px;
  min-width: 0;
  padding-inline: 6px;
}
.keypad-action {
  min-height: 0;
  flex: 1;
}
.ca-button {
  color: #c43f3f;
}

@container (max-width: 420px) {
  .calculator-keypad { column-gap: 8px; }
  .keypad-numbers { gap: 5px; }
  .keypad-actions { gap: 4px; }
  .keypad-button { min-height: 36px; border-radius: 9px; }
}
</style>
