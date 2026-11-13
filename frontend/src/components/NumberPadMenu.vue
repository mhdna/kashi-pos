<template>
  <k-menu
    v-model="open"
    :close-on-content-click="false"
    location="bottom center"
    :gap="6"
  >
    <template #activator="{ props: activatorProps }">
      <slot
        name="activator"
        :props="keyboardMode ? {} : activatorProps"
        :open="open"
        :set-ref="setActivatorRef"
        :on-keydown="onKeydown"
        :on-focus="onFocus"
        :on-blur="onBlur"
        :on-click="onClick"
        :keyboard-mode="keyboardMode"
      >
        <button
          :ref="setActivatorRef"
          v-bind="keyboardMode ? {} : activatorProps"
          :aria-label="ariaLabel"
          :class="{
            'number-pad-activator--error': tone === 'error',
            'number-pad-activator--keyboard': keyboardMode,
          }"
          :disabled="disabled"
          :style="{ minWidth: activatorMinWidthCh + 'ch' }"
          class="number-pad-activator"
          type="button"
          @click="onClick"
          @keydown="onKeydown"
          @focus="onFocus"
          @blur="onBlur"
        >
          <span v-if="prefix" class="number-pad-affix">{{ prefix }}</span>
          <span>{{ modelValue }}</span>
          <span v-if="suffix" class="number-pad-affix">{{ suffix }}</span>
        </button>
      </slot>
    </template>

    <k-card class="number-pad pa-2" rounded="lg">
      <!-- What is being changed ("Quantity", "Price"), and optionally whose
           (the item's name), above the number. -->
      <div v-if="title" class="number-pad-heading" data-test="number-pad-heading">
        <span class="number-pad-title">{{ title }}</span>
        <span v-if="subtitle" class="number-pad-subtitle">{{ subtitle }}</span>
      </div>
      <div class="number-pad-display">
        <span v-if="prefix" class="number-pad-display-affix">{{ prefix }}</span>
        <span>{{ draft || "0" }}</span>
        <span v-if="suffix" class="number-pad-display-affix">{{ suffix }}</span>
      </div>
      <div class="number-pad-grid">
        <k-btn
          v-for="digit in digits"
          :key="digit"
          class="number-pad-key"
          size="small"
          variant="tonal"
          @click="appendDigit(digit)"
        >
          {{ digit }}
        </k-btn>
        <k-btn class="number-pad-key" size="small" variant="text" @click="clear">C</k-btn>
        <k-btn class="number-pad-key" size="small" variant="tonal" @click="appendDigit('0')">
          0
        </k-btn>
        <k-btn
          v-if="allowDecimal"
          class="number-pad-key"
          size="small"
          variant="tonal"
          @click="appendDecimal"
        >
          .
        </k-btn>
        <k-btn
          class="number-pad-key"
          :class="{ 'number-pad-key--wide': allowDecimal }"
          color="error"
          size="small"
          variant="flat"
          data-test="number-pad-backspace"
          @click="backspace"
        >
          <k-icon icon="mdi-backspace-outline" size="20" />
        </k-btn>
      </div>
      <k-btn block class="number-pad-done mt-2" color="primary" size="small" variant="flat" @click="apply">
        Done
      </k-btn>
    </k-card>
  </k-menu>
</template>

<script setup lang="ts">
import {
  computed,
  inject,
  onBeforeUnmount,
  ref,
  watch,
  type ComponentPublicInstance,
  type Ref,
} from "vue";

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
    // Shown above the number in the pad: what is being changed, and whose.
    title?: string;
    subtitle?: string;
    disabled?: boolean;
    tone?: "default" | "error";
    allowDecimal?: boolean;
    decimalPlaces?: number;
    // When true, the popup never opens - the activator just takes focus and
    // digit/backspace/arrow keys edit the value directly, for the invoice
    // list's keyboard mode.
    keyboardMode?: boolean;
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
    allowDecimal: false,
    decimalPlaces: 2,
    keyboardMode: false,
  },
);

const emit = defineEmits<{
  "update:modelValue": [value: string | number];
  commit: [value: string | number];
  navigate: [direction: -1 | 1];
  "navigate-h": [direction: -1 | 1];
}>();

const digits = ["1", "2", "3", "4", "5", "6", "7", "8", "9"];
const open = ref(false);
const draft = ref("");
const replaceOnInput = ref(true);
const activatorEl = ref<HTMLElement | null>(null);
const dialogOpen = inject<Ref<boolean> | null>("dialogOpen", null);
let previousDialogState = false;

function setActivatorRef(element: Element | ComponentPublicInstance | null) {
  activatorEl.value = (element as HTMLElement | null) ?? null;
}

// The default activator button sizes to its own text content, so v-menu
// (anchored with no fixed `origin`) re-anchors horizontally on every
// keystroke as the digit count changes - visibly shifting the popup left
// and right while typing. Reserving width up front for the longest value
// this field can hold keeps the button's box (and the menu anchored to it)
// stationary regardless of how many digits are currently typed.
const activatorMinWidthCh = computed(() => {
  const wholeDigits = String(Math.trunc(Math.abs(props.max))).length;
  const decimalChars = props.allowDecimal ? props.decimalPlaces + 1 : 0;
  const digits = Math.min(props.maxLength, wholeDigits + decimalChars);
  const affixChars = (props.prefix.length || 0) + (props.suffix.length || 0);
  return digits + affixChars + (affixChars ? affixChars * 0.5 : 0);
});

// While the pad is open it stays where it opened: the menu places itself once,
// when it opens, and does not follow the activator as typing resizes it.

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
  if (props.allowDecimal && draft.value.includes(".")) {
    const fractionalPart = draft.value.split(".")[1] ?? "";
    if (!replaceOnInput.value && fractionalPart.length >= props.decimalPlaces) return;
  }
  const candidate =
    replaceOnInput.value || draft.value === "0" ? digit : `${draft.value}${digit}`;
  if (!props.preserveString && Number(candidate) > props.max) return;
  draft.value = candidate;
  replaceOnInput.value = false;
  emitDraft();
}

function appendDecimal() {
  if (!props.allowDecimal || (!replaceOnInput.value && draft.value.includes("."))) return;
  draft.value = replaceOnInput.value || !draft.value ? "0." : `${draft.value}.`;
  replaceOnInput.value = false;
  emitDraft();
}

function clear() {
  draft.value = "";
  replaceOnInput.value = false;
  emitDraft();
}

function backspace() {
  draft.value = draft.value.slice(0, -1);
  replaceOnInput.value = false;
  emitDraft();
}

function normalizedValue() {
  if (props.preserveString) return draft.value;
  const raw = Number(draft.value) || 0;
  const factor = 10 ** props.decimalPlaces;
  const numeric = props.allowDecimal ? Math.round(raw * factor) / factor : Math.trunc(raw);
  return Math.min(props.max, Math.max(props.min, numeric));
}

function emitDraft() {
  emit("update:modelValue", normalizedValue());
}

function apply() {
  const value = normalizedValue();
  emit("update:modelValue", value);
  emit("commit", value);
  open.value = false;
}

// WKWebView (the Wails webview on macOS) doesn't give a <button> keyboard
// focus on mouse click the way other browsers do - only Tab does - so
// keyboard mode has to focus it explicitly here.
function onClick(event: MouseEvent) {
  if (props.keyboardMode) (event.currentTarget as HTMLElement | null)?.focus();
}

function navigate(direction: -1 | 1) {
  // In keyboard mode the popup never opens, so draft is seeded on focus
  // instead - it's always safe to apply it here.
  if (open.value || props.keyboardMode) apply();
  emit("navigate", direction);
}

function navigateHorizontal(direction: -1 | 1) {
  if (open.value || props.keyboardMode) apply();
  emit("navigate-h", direction);
}

function onFocus() {
  if (!props.keyboardMode) return;
  draft.value = String(props.modelValue);
  replaceOnInput.value = true;
}

function onBlur() {
  if (props.keyboardMode) apply();
}

function onKeydown(event: KeyboardEvent) {
  if (props.keyboardMode) {
    if (/^[0-9]$/.test(event.key)) {
      event.preventDefault();
      appendDigit(event.key);
      return;
    }
    if (event.key === "." && props.allowDecimal) {
      event.preventDefault();
      appendDecimal();
      return;
    }
    if (event.key === "Backspace") {
      event.preventDefault();
      backspace();
      return;
    }
    if (event.key === "Delete") {
      event.preventDefault();
      clear();
      return;
    }
    if (event.key === "Enter") {
      event.preventDefault();
      apply();
      return;
    }
    if (event.key === "ArrowLeft") {
      event.preventDefault();
      navigateHorizontal(-1);
      return;
    }
    if (event.key === "ArrowRight") {
      event.preventDefault();
      navigateHorizontal(1);
      return;
    }
    if (event.key === "ArrowUp") {
      event.preventDefault();
      navigate(-1);
      return;
    }
    if (event.key === "ArrowDown") {
      event.preventDefault();
      navigate(1);
      return;
    }
    return;
  }
  if (event.key === "Enter") {
    event.preventDefault();
    open.value = true;
    return;
  }
  if (event.key === "ArrowUp") {
    event.preventDefault();
    navigate(-1);
    return;
  }
  if (event.key === "ArrowDown") {
    event.preventDefault();
    navigate(1);
    return;
  }
}

function focus() {
  activatorEl.value?.focus();
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
  font-size: var(--kashi-font-size-xl);
  font-variant-numeric: tabular-nums;
  font-weight: 700;
  justify-content: center;
  line-height: 1;
  min-height: 32px;
  padding: 2px 5px;
}

.number-pad-activator:hover,
.number-pad-activator:focus-visible {
  background: rgba(var(--k-theme-primary), 0.09);
  outline: none;
}

.number-pad-activator:disabled {
  cursor: default;
  opacity: 1;
}

.number-pad-activator--error {
  color: rgb(var(--k-theme-error));
}

.number-pad-activator--keyboard:focus {
  background: rgba(var(--k-theme-primary), 0.14);
  outline: 2px solid rgba(var(--k-theme-primary), 0.5);
  outline-offset: 1px;
}

.number-pad-affix,
.number-pad-display-affix {
  font-size: var(--kashi-font-size-xs);
  font-weight: 600;
  margin: 0 2px;
  opacity: 0.7;
}

.number-pad {
  width: 176px;
}

.number-pad-heading {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1px;
  padding: 2px 4px 0;
  text-align: center;
}
.number-pad-title {
  font-size: 0.6875rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgba(var(--k-theme-on-surface), 0.6);
}
.number-pad-subtitle {
  max-width: 100%;
  overflow: hidden;
  font-size: 0.75rem;
  color: rgba(var(--k-theme-on-surface), 0.7);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.number-pad-display {
  align-items: baseline;
  display: flex;
  font-size: var(--kashi-font-size-3xl);
  font-weight: 700;
  justify-content: center;
  min-height: 34px;
  padding: 8px 0;
}

/* The currency sign beside the number being typed, larger than the small
   one on the page's own field (.number-pad-affix). */
.number-pad-display .number-pad-display-affix {
  font-size: var(--kashi-font-size-xl);
}

.number-pad-grid {
  display: grid;
  gap: 4px;
  grid-template-columns: repeat(3, 1fr);
}

.number-pad-key--wide {
  grid-column: 1 / -1;
}

.number-pad-key {
  font-size: var(--kashi-font-size-xl);
  height: 42px;
  min-width: 0;
  padding: 0;
}

.number-pad-done {
  height: 42px !important;
}
</style>
