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
          ref="activator"
          v-bind="activatorProps"
          :aria-label="ariaLabel"
          :class="{ 'number-pad-activator--error': tone === 'error' }"
          :disabled="disabled"
          class="number-pad-activator"
          type="button"
          @keydown.enter.stop.prevent="open = true"
          @keydown.up.stop.prevent="navigate(-1)"
          @keydown.down.stop.prevent="navigate(1)"
        >
          <span v-if="prefix" class="number-pad-affix">{{ prefix }}</span>
          <span>{{ modelValue }}</span>
          <span v-if="suffix" class="number-pad-affix">{{ suffix }}</span>
        </button>
      </slot>
    </template>

    <v-card class="number-pad pa-2" rounded="lg">
      <div class="number-pad-display">
        <span v-if="prefix" class="number-pad-display-affix">{{ prefix }}</span>
        <span>{{ draft || "0" }}</span>
        <span v-if="suffix" class="number-pad-display-affix">{{ suffix }}</span>
      </div>
      <div class="number-pad-grid">
        <v-btn
          v-for="digit in digits"
          :key="digit"
          class="number-pad-key"
          size="small"
          variant="tonal"
          @click="appendDigit(digit)"
        >
          {{ digit }}
        </v-btn>
        <v-btn class="number-pad-key" size="small" variant="text" @click="clear">C</v-btn>
        <v-btn class="number-pad-key" size="small" variant="tonal" @click="appendDigit('0')">
          0
        </v-btn>
        <v-btn class="number-pad-key" size="small" variant="text" @click="backspace">⌫</v-btn>
      </div>
      <v-btn block class="mt-2" color="primary" size="small" variant="flat" @click="apply">
        Done
      </v-btn>
    </v-card>
  </v-menu>
</template>

<script setup lang="ts">
import { inject, onBeforeUnmount, ref, watch, type Ref } from "vue";

const props = withDefaults(
  defineProps<{
    modelValue: string | number;
    min?: number;
    max?: number;
    maxLength?: number;
    preserveString?: boolean;
    replaceOnOpen?: boolean;
    prefix?: string;
    suffix?: string;
    ariaLabel?: string;
    disabled?: boolean;
    tone?: "default" | "error";
  }>(),
  {
    min: 0,
    max: 999,
    maxLength: 12,
    preserveString: false,
    replaceOnOpen: true,
    prefix: "",
    suffix: "",
    ariaLabel: "Open number pad",
    disabled: false,
    tone: "default",
  },
);

const emit = defineEmits<{
  "update:modelValue": [value: string | number];
  commit: [value: string | number];
  navigate: [direction: -1 | 1];
}>();

const digits = ["1", "2", "3", "4", "5", "6", "7", "8", "9"];
const open = ref(false);
const draft = ref("");
const replaceOnInput = ref(true);
const activator = ref<HTMLButtonElement | null>(null);
const dialogOpen = inject<Ref<boolean> | null>("dialogOpen", null);
let previousDialogState = false;

watch(open, (value) => {
  if (value) {
    draft.value = String(props.modelValue);
    replaceOnInput.value = props.replaceOnOpen;
    if (dialogOpen) {
      previousDialogState = dialogOpen.value;
      dialogOpen.value = true;
    }
  } else if (dialogOpen) {
    dialogOpen.value = previousDialogState;
  }
});

function appendDigit(digit: string) {
  if (!replaceOnInput.value && draft.value.length >= props.maxLength) return;
  const candidate =
    replaceOnInput.value || draft.value === "0" ? digit : `${draft.value}${digit}`;
  if (!props.preserveString && Number(candidate) > props.max) return;
  draft.value = candidate;
  replaceOnInput.value = false;
}

function clear() {
  draft.value = "";
  replaceOnInput.value = false;
}

function backspace() {
  draft.value = draft.value.slice(0, -1);
  replaceOnInput.value = false;
}

function normalizedValue() {
  if (props.preserveString) return draft.value;
  const numeric = Math.trunc(Number(draft.value) || 0);
  return Math.min(props.max, Math.max(props.min, numeric));
}

function apply() {
  const value = normalizedValue();
  emit("update:modelValue", value);
  emit("commit", value);
  open.value = false;
}

function navigate(direction: -1 | 1) {
  if (open.value) apply();
  emit("navigate", direction);
}

function focus() {
  activator.value?.focus();
}

onBeforeUnmount(() => {
  if (dialogOpen && open.value) dialogOpen.value = previousDialogState;
});

defineExpose({ focus });
</script>

<style scoped>
.number-pad-activator {
  align-items: baseline;
  background: transparent;
  border: 0;
  border-radius: 6px;
  color: inherit;
  cursor: pointer;
  display: inline-flex;
  font: inherit;
  font-size: 1.15rem;
  font-weight: 700;
  justify-content: center;
  line-height: 1;
  min-height: 32px;
  padding: 2px 5px;
}

.number-pad-activator:hover,
.number-pad-activator:focus-visible {
  background: rgba(var(--v-theme-primary), 0.09);
  outline: none;
}

.number-pad-activator:disabled {
  cursor: default;
  opacity: 1;
}

.number-pad-activator--error {
  color: rgb(var(--v-theme-error));
}

.number-pad-affix,
.number-pad-display-affix {
  font-size: 0.72em;
  font-weight: 600;
  margin: 0 2px;
  opacity: 0.7;
}

.number-pad {
  width: 176px;
}

.number-pad-display {
  align-items: baseline;
  display: flex;
  font-size: 1.4rem;
  font-weight: 700;
  justify-content: center;
  min-height: 34px;
}

.number-pad-grid {
  display: grid;
  gap: 4px;
  grid-template-columns: repeat(3, 1fr);
}

.number-pad-key {
  font-size: 1rem;
  height: 36px;
  min-width: 0;
  padding: 0;
}
</style>
