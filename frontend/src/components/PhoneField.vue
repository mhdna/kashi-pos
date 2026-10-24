<template>
  <v-text-field
    ref="field"
    :model-value="display"
    :placeholder="placeholder"
    variant="outlined"
    density="compact"
    inputmode="none"
    autocomplete="off"
    v-bind="$attrs"
    @beforeinput="onBeforeInput"
    @keydown="onKeydown"
    @focus="emit('focus')"
  >
    <template v-if="$slots.prepend" #prepend><slot name="prepend" /></template>
  </v-text-field>
</template>

<script setup lang="ts">
import { computed, nextTick, ref } from "vue";
import { formatLebanesePhone } from "@/lib/phone";

// A phone number edited digit by digit, from the keyboard or a PhonePad
// (press()). The value is digits, with a leading "+" kept for an
// international number; a plain local number shows formatted as
// "(78) 928-747". The caret is tracked by digit, so the formatting
// characters never get in the way of inserting, deleting or moving.

defineOptions({ inheritAttrs: false });

const props = withDefaults(defineProps<{ modelValue: string; placeholder?: string; maxDigits?: number }>(), {
  placeholder: "Phone Number",
  maxDigits: 15,
});
const emit = defineEmits<{ "update:modelValue": [value: string]; focus: [] }>();

const field = ref<any>(null);
const plus = computed(() => props.modelValue.trim().startsWith("+"));
const digits = computed(() => props.modelValue.replace(/\D/g, ""));
const display = computed(() => (plus.value ? `+${digits.value}` : formatLebanesePhone(digits.value)));

function input(): HTMLInputElement | null {
  return field.value?.$el?.querySelector("input") ?? null;
}

// Digits in the display before a character position.
function digitIndexAt(position: number) {
  return (display.value.slice(0, position).match(/\d/g) || []).length;
}

function selection(): [number, number] {
  const el = input();
  const end = digits.value.length;
  if (!el || document.activeElement !== el) return [end, end];
  return [digitIndexAt(el.selectionStart ?? display.value.length), digitIndexAt(el.selectionEnd ?? display.value.length)];
}

function commit(next: string, caret: number) {
  emit("update:modelValue", (plus.value ? "+" : "") + next);
  void nextTick(() => placeCaret(caret));
}

// Puts the caret just after the caret-th digit of the (new) display.
function placeCaret(caret: number) {
  const el = input();
  if (!el) return;
  el.focus();
  const text = display.value;
  let position = 0;
  let seen = 0;
  if (caret > 0) {
    while (position < text.length && seen < caret) {
      if (/\d/.test(text.charAt(position))) seen++;
      position++;
    }
  } else {
    while (position < text.length && !/\d/.test(text.charAt(position))) position++;
  }
  el.setSelectionRange(position, position);
}

function insert(typed: string) {
  const add = typed.replace(/\D/g, "");
  if (!add) return;
  const [start, end] = selection();
  const next = digits.value.slice(0, start) + add + digits.value.slice(end);
  if (next.length > props.maxDigits) return;
  commit(next, start + add.length);
}

function remove(direction: "back" | "forward") {
  let [start, end] = selection();
  if (start === end) {
    if (direction === "back") {
      if (start === 0) return;
      start--;
    } else {
      if (end >= digits.value.length) return;
      end++;
    }
  }
  commit(digits.value.slice(0, start) + digits.value.slice(end), start);
}

function move(step: number) {
  const [start, end] = selection();
  const from = step < 0 ? start : end;
  placeCaret(start !== end ? from : Math.min(Math.max(from + step, 0), digits.value.length));
}

// Every edit goes through insert/remove so the stored digits stay the source
// of truth; the browser never edits the formatted text itself.
function onBeforeInput(event: InputEvent) {
  event.preventDefault();
  if (event.inputType === "deleteContentBackward") remove("back");
  else if (event.inputType === "deleteContentForward") remove("forward");
  else if (event.data) insert(event.data);
  else if (event.inputType === "insertFromPaste") insert(event.dataTransfer?.getData("text") ?? "");
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === "Backspace" || event.key === "Delete") {
    event.preventDefault();
    remove(event.key === "Backspace" ? "back" : "forward");
  }
}

// PhonePad keys: a digit, "back", "clear", "left" or "right".
function press(key: string) {
  if (/^\d$/.test(key)) insert(key);
  else if (key === "back") remove("back");
  else if (key === "clear") commit("", 0);
  else if (key === "left") move(-1);
  else if (key === "right") move(1);
}

function focus() {
  placeCaret(digits.value.length);
}

defineExpose({ press, focus });
</script>
