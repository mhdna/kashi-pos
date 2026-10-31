<template>
  <!-- mousedown.prevent keeps the field focused, with its caret and
       selection where they were, while a key is pressed. -->
  <div class="inline-keyboard" @mousedown.prevent>
    <div
      v-for="(row, rowIndex) in visibleRows"
      :key="rowIndex"
      class="inline-keyboard-row"
    >
      <k-btn
        v-for="letter in row"
        :key="letter"
        class="inline-keyboard-key"
        size="small"
        variant="tonal"
        @click="append(letter)"
      >
        {{ shift ? letter : letter.toLowerCase() }}
      </k-btn>
    </div>
    <div class="inline-keyboard-actions">
      <template v-if="input">
        <k-btn class="inline-keyboard-move" size="small" variant="text" aria-label="Caret left" @click="move(-1)">
          <k-icon icon="mdi-chevron-left" size="28" />
        </k-btn>
        <k-btn class="inline-keyboard-move" size="small" variant="text" aria-label="Caret right" @click="move(1)">
          <k-icon icon="mdi-chevron-right" size="28" />
        </k-btn>
      </template>
      <k-btn size="small" variant="text" @click="append('-')">-</k-btn>
      <k-btn class="inline-keyboard-space" size="small" variant="tonal" @click="append(' ')">
        Space
      </k-btn>
      <k-btn class="inline-keyboard-clear" size="small" variant="text" @click="clear">C</k-btn>
      <k-btn class="inline-keyboard-backspace" size="small" variant="text" @click="backspace">⌫</k-btn>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref } from "vue";
import { replaceRange } from "@/lib/caret";

const props = withDefaults(
  defineProps<{
    modelValue: string;
    maxLength?: number;
    layout?: "text" | "code";
    // The field being typed into. Given, keys type at its caret (replacing
    // a selection) and the caret keys show; without it they add at the end.
    input?: HTMLInputElement | null;
  }>(),
  {
    maxLength: 80,
    layout: "text",
    input: null,
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

// The field's selection, or the end of the text when it isn't focused (or
// there's no field), which is where the keys used to type.
function selection(): [number, number] {
  const el = props.input;
  const end = props.modelValue.length;
  if (!el || document.activeElement !== el) return [end, end];
  return [Math.min(el.selectionStart ?? end, end), Math.min(el.selectionEnd ?? end, end)];
}

function commit(value: string, caret: number) {
  emit("update:modelValue", value);
  const el = props.input;
  if (!el) return;
  // After the field shows the new value.
  void nextTick(() => {
    el.focus({ preventScroll: true });
    el.setSelectionRange(caret, caret);
  });
}

function append(character: string) {
  const [start, end] = selection();
  if (props.modelValue.length - (end - start) >= props.maxLength) return;
  const before = props.modelValue.slice(0, start);
  const wordStart = !before || /[\s'-]$/.test(before);
  const value =
    character === " "
      ? character
      : wordStart || shift.value
        ? character.toUpperCase()
        : character.toLowerCase();
  const next = replaceRange(props.modelValue, start, end, value);
  commit(next.value, next.caret);
  if (character !== " ") shift.value = false;
}

function clear() {
  commit("", 0);
  shift.value = false;
}

function backspace() {
  let [start, end] = selection();
  if (start === end) {
    if (start === 0) return;
    start--;
  }
  const next = replaceRange(props.modelValue, start, end, "");
  commit(next.value, next.caret);
}

// A selection collapses to its side; otherwise the caret steps one character.
function move(step: number) {
  const el = props.input;
  if (!el) return;
  const [start, end] = selection();
  const caret = start !== end ? (step < 0 ? start : end) : Math.min(Math.max(start + step, 0), props.modelValue.length);
  el.focus({ preventScroll: true });
  el.setSelectionRange(caret, caret);
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
  background-color: rgba(var(--k-theme-on-surface), 0.1);
}

.inline-keyboard-row {
  display: flex;
  gap: 6px;
  justify-content: center;
  flex-wrap: nowrap;
  margin-bottom: 4px;
}

.inline-keyboard-key {
  /* Every key in a row shares the width equally, so a row always fits its
     container exactly - it can never overflow and get clipped, and on a wide
     container the max-width keeps the keys from ballooning. The !important
     rules beat the kit's own .k-btn sizing (min-width / horizontal padding),
     which otherwise forces the keys wider than their slot. */
  flex: 1 1 0;
  min-width: 0 !important;
  max-width: 72px;
  font-size: var(--kashi-font-size-2xl);
  height: 48px;
  padding: 0 !important;
}

.inline-keyboard-actions {
  display: flex;
  gap: 4px;
  justify-content: center;
  font-size: var(--kashi-font-size-lg);
}

.inline-keyboard-actions > .k-btn {
  height: 48px;
  min-height: 48px;
  font-size: var(--kashi-font-size-2xl);
}

.inline-keyboard-space {
  flex: 3 1 auto;
  max-width: 300px;
}

.inline-keyboard-backspace {
  min-width: 90px;
}

.inline-keyboard-move {
  min-width: 56px;
  background-color: rgba(var(--k-theme-on-surface), 0.06);
}
</style>
