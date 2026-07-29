<template>
  <v-menu
    v-model="open"
    :close-on-content-click="false"
    location="bottom"
    offset="6"
  >
    <template #activator="{ props: activatorProps }">
      <slot name="activator" :props="activatorProps" :open="open">
        <button
          v-bind="activatorProps"
          :aria-label="ariaLabel"
          class="popup-keyboard-activator"
          type="button"
        >
          {{ modelValue || "ABC" }}
        </button>
      </slot>
    </template>

    <v-card class="popup-keyboard pa-2" rounded="lg">
      <div class="popup-keyboard-display">{{ draft || placeholder }}</div>
      <div
        v-for="(row, rowIndex) in visibleRows"
        :key="rowIndex"
        class="popup-keyboard-row"
      >
        <v-btn
          v-for="letter in row"
          :key="letter"
          class="popup-keyboard-key"
          size="small"
          variant="tonal"
          @click="append(letter)"
        >
          {{ uppercase ? letter : letter.toLowerCase() }}
        </v-btn>
      </div>
      <div class="popup-keyboard-actions">
        <v-btn size="small" variant="text" @click="clear">C</v-btn>
        <v-btn size="small" variant="text" @click="uppercase = !uppercase">
          {{ uppercase ? "ABC" : "abc" }}
        </v-btn>
        <v-btn class="popup-keyboard-space" size="small" variant="tonal" @click="append(' ')">
          Space
        </v-btn>
        <v-btn v-if="layout === 'code'" size="small" variant="text" @click="append('/')">/</v-btn>
        <v-btn v-if="layout === 'code'" size="small" variant="text" @click="append('.')">.</v-btn>
        <v-btn size="small" variant="text" @click="append('-')">-</v-btn>
        <v-btn size="small" variant="text" @click="append(`'`)">'</v-btn>
        <v-btn size="small" variant="text" @click="backspace">⌫</v-btn>
      </div>
      <v-btn block class="mt-2" color="primary" size="small" variant="flat" @click="apply">
        Done
      </v-btn>
    </v-card>
  </v-menu>
</template>

<script setup lang="ts">
import { computed, inject, onBeforeUnmount, ref, watch, type Ref } from "vue";

const props = withDefaults(
  defineProps<{
    modelValue: string;
    maxLength?: number;
    ariaLabel?: string;
    layout?: "text" | "code";
    placeholder?: string;
  }>(),
  {
    maxLength: 80,
    ariaLabel: "Open keyboard",
    layout: "text",
    placeholder: "Enter text",
  },
);

const emit = defineEmits<{
  "update:modelValue": [value: string];
  commit: [value: string];
}>();

const rows = [
  ["Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"],
  ["A", "S", "D", "F", "G", "H", "J", "K", "L"],
  ["Z", "X", "C", "V", "B", "N", "M"],
];
const numberRow = ["1", "2", "3", "4", "5", "6", "7", "8", "9", "0"];
const visibleRows = computed(() => (props.layout === "code" ? [numberRow, ...rows] : rows));
const open = ref(false);
const draft = ref("");
const uppercase = ref(true);
const dialogOpen = inject<Ref<boolean> | null>("dialogOpen", null);
let previousDialogState = false;

watch(open, (value) => {
  if (value) {
    draft.value = props.modelValue;
    uppercase.value = !draft.value || /\s$/.test(draft.value);
    if (dialogOpen) {
      previousDialogState = dialogOpen.value;
      dialogOpen.value = true;
    }
  } else if (dialogOpen) {
    dialogOpen.value = previousDialogState;
  }
});

function append(character: string) {
  if (draft.value.length >= props.maxLength) return;
  draft.value += character === " " ? character : uppercase.value ? character : character.toLowerCase();
  emitDraft();
}

function clear() {
  draft.value = "";
  emitDraft();
}

function backspace() {
  draft.value = draft.value.slice(0, -1);
  emitDraft();
}

function emitDraft() {
  emit("update:modelValue", draft.value);
}

function apply() {
  const value = draft.value.trim().replace(/\s+/g, " ");
  emit("update:modelValue", value);
  emit("commit", value);
  open.value = false;
}

onBeforeUnmount(() => {
  if (dialogOpen && open.value) dialogOpen.value = previousDialogState;
});
</script>

<style scoped>
.popup-keyboard-activator {
  background: transparent;
  border: 0;
  border-radius: 6px;
  cursor: pointer;
  font-size: 1rem;
  font-weight: 700;
  min-height: 32px;
  padding: 2px 8px;
}

.popup-keyboard-activator:hover,
.popup-keyboard-activator:focus-visible {
  background: rgba(var(--v-theme-primary), 0.09);
  outline: none;
}

.popup-keyboard {
  width: 390px;
}

.popup-keyboard-display {
  font-size: 1.1rem;
  font-weight: 600;
  min-height: 36px;
  overflow: hidden;
  padding: 6px 8px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.popup-keyboard-row {
  display: flex;
  gap: 3px;
  justify-content: center;
  margin-bottom: 3px;
}

.popup-keyboard-key {
  font-size: 0.9rem;
  height: 34px;
  min-width: 32px;
  padding: 0;
}

.popup-keyboard-actions {
  display: flex;
  gap: 3px;
}

.popup-keyboard-space {
  flex: 1 1 auto;
}
</style>
