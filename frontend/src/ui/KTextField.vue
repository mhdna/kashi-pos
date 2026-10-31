<script setup lang="ts">
import { computed, inject, nextTick, onBeforeUnmount, onMounted, ref, useAttrs } from "vue";
import KField from "./KField.vue";
import { firstRuleError, normaliseMessages, type Rule } from "./fieldRules";
import { formKey } from "./keys";

defineOptions({ inheritAttrs: false });

const props = defineProps<{
  modelValue?: unknown;
  variant?: string;
  density?: string;
  label?: string;
  placeholder?: string;
  hint?: string;
  persistentHint?: boolean;
  errorMessages?: string | string[];
  rules?: Rule[];
  hideDetails?: boolean | string;
  prependInnerIcon?: string;
  appendInnerIcon?: string;
  clearable?: boolean;
  prefix?: string;
  suffix?: string;
  type?: string;
  disabled?: boolean;
  autofocus?: boolean;
  active?: boolean;
  multiline?: boolean;
  autoGrow?: boolean;
  rows?: string | number;
  counter?: boolean;
  maxlength?: string | number;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: string];
  "click:prepend-inner": [event: MouseEvent];
  "click:append-inner": [event: MouseEvent];
  "click:clear": [event: MouseEvent];
}>();
defineSlots<{ prepend?(): unknown; append?(): unknown; "prepend-inner"?(): unknown; "append-inner"?(): unknown }>();

const attrs = useAttrs();
const input = ref<HTMLInputElement | HTMLTextAreaElement | null>(null);
const focused = ref(false);
const validated = ref(false);

// class/style belong to the outer element; everything else (listeners, min,
// max, step, inputmode, autocomplete...) goes to the real <input>.
const rootAttrs = computed(() => ({ class: attrs.class, style: attrs.style }));
const inputAttrs = computed(() => {
  const { class: _c, style: _s, ...rest } = attrs;
  return rest;
});

const text = computed(() => (props.modelValue === undefined || props.modelValue === null ? "" : String(props.modelValue)));
const dirty = computed(() => text.value !== "");
const floating = computed(() => focused.value || dirty.value || !!props.active || props.type === "date" || props.type === "month");
const ruleError = computed(() => (validated.value ? firstRuleError(props.rules, props.modelValue) : ""));
const messages = computed(() => normaliseMessages(props.errorMessages));
const hasError = computed(() => messages.value.length > 0 || !!ruleError.value);
const message = computed(() => {
  if (messages.value.length) return messages.value[0];
  if (ruleError.value) return ruleError.value.trim();
  if (props.hint && (focused.value || props.persistentHint)) return props.hint;
  return "";
});
const showPlaceholder = computed(() => !props.label || floating.value);
const counter = computed(() => (props.counter ? `${text.value.length}${props.maxlength ? ` / ${props.maxlength}` : ""}` : undefined));

function onInput(event: Event) {
  const el = event.target as HTMLInputElement;
  emit("update:modelValue", el.value);
  if (props.autoGrow) grow();
}
function onBlur() {
  focused.value = false;
  if (props.rules?.length) validated.value = true;
}
function grow() {
  const el = input.value as HTMLTextAreaElement | null;
  if (!el) return;
  el.style.height = "auto";
  el.style.height = `${el.scrollHeight}px`;
}
function clear(event: MouseEvent) {
  emit("update:modelValue", "");
  emit("click:clear", event);
  input.value?.focus();
}
function validate() {
  validated.value = true;
  return !firstRuleError(props.rules, props.modelValue) && messages.value.length === 0;
}
function focus(options?: FocusOptions) {
  input.value?.focus(options);
}
function blur() {
  input.value?.blur();
}
function select() {
  input.value?.select();
}

const form = inject(formKey, null);
let unregister: (() => void) | undefined;
onMounted(() => {
  if (props.rules?.length) unregister = form?.register(validate);
  if (props.autoGrow) void nextTick(grow);
  if (props.autofocus) requestAnimationFrame(() => input.value?.focus());
});
onBeforeUnmount(() => unregister?.());
defineExpose({ focus, blur, select, validate, input });
</script>

<template>
  <KField
    v-bind="rootAttrs"
    :variant="variant"
    :density="density"
    :label="label"
    :focused="focused"
    :floating="floating"
    :error="hasError"
    :disabled="disabled"
    :multiline="multiline"
    :prepend-inner-icon="prependInnerIcon"
    :append-inner-icon="appendInnerIcon"
    :show-clear="clearable && dirty && !disabled"
    :message="message"
    :message-is-error="hasError"
    :hide-details="hideDetails"
    :prefix="prefix"
    :suffix="suffix"
    :counter="counter"
    @click:prepend-inner="emit('click:prepend-inner', $event)"
    @click:append-inner="emit('click:append-inner', $event)"
    @click:clear="clear"
  >
    <template v-if="$slots.prepend" #prepend><slot name="prepend" /></template>
    <template v-if="$slots.append" #append><slot name="append" /></template>
    <template v-if="$slots['prepend-inner']" #prepend-inner><slot name="prepend-inner" /></template>
    <template v-if="$slots['append-inner']" #append-inner><slot name="append-inner" /></template>
    <textarea
      v-if="multiline"
      ref="input"
      v-bind="inputAttrs"
      class="k-field__input"
      :value="text"
      :rows="rows ?? 1"
      :maxlength="maxlength"
      :placeholder="showPlaceholder ? placeholder : undefined"
      :disabled="disabled"
      @input="onInput"
      @focus="focused = true"
      @blur="onBlur"
    />
    <input
      v-else
      ref="input"
      v-bind="inputAttrs"
      class="k-field__input"
      :type="type ?? 'text'"
      :value="text"
      :maxlength="maxlength"
      :placeholder="showPlaceholder ? placeholder : undefined"
      :disabled="disabled"
      @input="onInput"
      @focus="focused = true"
      @blur="onBlur"
    />
  </KField>
</template>
