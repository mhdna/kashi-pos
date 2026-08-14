<template>
  <div class="inline-keyboard">
    <div
      v-for="(row, rowIndex) in visibleRows"
      :key="rowIndex"
      class="inline-keyboard-row"
    >
      <v-btn
        v-for="letter in row"
        :key="letter"
        class="inline-keyboard-key"
        size="small"
        variant="tonal"
        @click="append(letter)"
      >
        {{ shift ? letter : letter.toLowerCase() }}
      </v-btn>
    </div>
    <div class="inline-keyboard-actions">
      <v-btn size="small" variant="text" @click="append('-')">-</v-btn>
      <v-btn class="inline-keyboard-space" size="small" variant="tonal" @click="append(' ')">
        Space
      </v-btn>
      <v-btn class="inline-keyboard-backspace" size="small" variant="text" @click="backspace">⌫</v-btn>
      <v-btn class="inline-keyboard-clear" size="small" variant="text" @click="clear">C</v-btn>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";

const props = withDefaults(
  defineProps<{
    modelValue: string;
    maxLength?: number;
    layout?: "text" | "code";
  }>(),
  {
    maxLength: 80,
    layout: "text",
  },
);

const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();

const rows = [
  ["Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"],
  ["A", "S", "D", "F", "G", "H", "J", "K", "L"],
  ["Z", "X", "C", "V", "B", "N", "M"],
];
const numberRow = ["1", "2", "3", "4", "5", "6", "7", "8", "9", "0"];
const visibleRows = computed(() => (props.layout === "code" ? [numberRow, ...rows] : rows));
const shift = ref(false);

function append(character: string) {
  if (props.modelValue.length >= props.maxLength) return;
  const wordStart = !props.modelValue || /[\s'-]$/.test(props.modelValue);
  const value =
    character === " "
      ? character
      : wordStart || shift.value
        ? character.toUpperCase()
        : character.toLowerCase();
  emit("update:modelValue", props.modelValue + value);
  if (character !== " ") shift.value = false;
}

function clear() {
  emit("update:modelValue", "");
  shift.value = false;
}

function backspace() {
  emit("update:modelValue", props.modelValue.slice(0, -1));
}
</script>

<style scoped>
.inline-keyboard {
  width: 100%;
}
.inline-keyboard-clear {
  background-color: #c43f3f;
  color: #fff;
}
.inline-keyboard-backspace {
  background-color: rgba(var(--v-theme-on-surface), 0.1);
}

.inline-keyboard-row {
  display: flex;
  gap: 4px;
  justify-content: center;
  margin-bottom: 4px;
}

.inline-keyboard-key {
  flex: 0 0 56px;
  font-size: var(--kashi-font-size-lg);
  height: 40px;
  width: 56px;
  padding: 0;
}

.inline-keyboard-actions {
  display: flex;
  gap: 4px;
  justify-content: center;
  font-size: var(--kashi-font-size-lg);
}

.inline-keyboard-actions > .v-btn {
  height: 40px;
  min-height: 40px;
}

.inline-keyboard-space {
  flex: 3 1 auto;
  max-width: 300px;
}
</style>
