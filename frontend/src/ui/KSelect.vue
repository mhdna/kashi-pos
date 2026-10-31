<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref, useAttrs } from "vue";
import KChip from "./KChip.vue";
import KField from "./KField.vue";
import KIcon from "./KIcon";
import KList from "./KList.vue";
import KListItem from "./KListItem.vue";
import KMenu from "./KMenu.vue";
import KProgressCircular from "./KProgressCircular.vue";
import { firstRuleError, normaliseMessages, type Rule } from "./fieldRules";
import { isSelected, normaliseItems, toggleSelection, type Item } from "./items";
import { formKey } from "./keys";

defineOptions({ inheritAttrs: false });
const props = defineProps<{
  modelValue?: unknown;
  items?: readonly unknown[];
  itemTitle?: string | ((item: any) => unknown);
  itemValue?: string | ((item: any) => unknown);
  multiple?: boolean;
  chips?: boolean;
  closableChips?: boolean;
  hideSelected?: boolean;
  clearable?: boolean;
  label?: string;
  placeholder?: string;
  variant?: string;
  density?: string;
  hideDetails?: boolean | string;
  hint?: string;
  persistentHint?: boolean;
  disabled?: boolean;
  loading?: boolean;
  prependInnerIcon?: string;
  rules?: Rule[];
  errorMessages?: string | string[];
  active?: boolean;
  menuProps?: unknown;
}>();
const emit = defineEmits<{ "update:modelValue": [value: any] }>();
defineSlots<{
  item?(scope: { props: Record<string, unknown>; item: Item; index: number }): unknown;
  chip?(scope: { props: Record<string, unknown>; item: Item; index: number }): unknown;
  "prepend-inner"?(): unknown;
}>();

const attrs = useAttrs();
const rootAttrs = computed(() => ({ class: attrs.class, style: attrs.style }));
const open = ref(false);
const focused = ref(false);
const hover = ref(-1);
const validated = ref(false);
const field = ref<{ $el: HTMLElement } | null>(null);
const anchorEl = ref<HTMLElement | null>(null);

const options = computed(() => normaliseItems(props.items, props.itemTitle, props.itemValue));
const selected = computed<Item[]>(() => {
  const values = props.multiple ? (Array.isArray(props.modelValue) ? props.modelValue : []) : props.modelValue === undefined || props.modelValue === null || props.modelValue === "" ? [] : [props.modelValue];
  return values.map((v) => options.value.find((o) => o.value === v) ?? { title: String(v), value: v, raw: v });
});
const visibleOptions = computed(() => (props.hideSelected ? options.value.filter((o) => !isSelected(props.modelValue, o.value, !!props.multiple)) : options.value));
const hasValue = computed(() => selected.value.length > 0);
const floating = computed(() => open.value || focused.value || hasValue.value || !!props.active);

const ruleError = computed(() => (validated.value ? firstRuleError(props.rules, props.modelValue) : ""));
const messages = computed(() => normaliseMessages(props.errorMessages));
const hasError = computed(() => messages.value.length > 0 || !!ruleError.value);
const message = computed(() => messages.value[0] ?? (ruleError.value.trim() || (props.hint && (focused.value || props.persistentHint) ? props.hint : "")));

function onFieldClick(event: MouseEvent) {
  if ((event.target as HTMLElement).closest(".k-field__clear")) return;
  toggleMenu();
}
function toggleMenu() {
  if (props.disabled) return;
  anchorEl.value = field.value?.$el.querySelector<HTMLElement>(".k-field") ?? null;
  open.value = !open.value;
  hover.value = -1;
}
function pick(item: Item) {
  emit("update:modelValue", toggleSelection(props.modelValue, item.value, !!props.multiple));
  if (!props.multiple) open.value = false;
}
function remove(item: Item) {
  emit("update:modelValue", toggleSelection(props.modelValue, item.value, true));
}
function clear() {
  emit("update:modelValue", props.multiple ? [] : null);
}
function onBlur() {
  focused.value = false;
  if (props.rules?.length) validated.value = true;
}
function onKey(event: KeyboardEvent) {
  const list = visibleOptions.value;
  if (event.key === "ArrowDown" || event.key === "ArrowUp") {
    event.preventDefault();
    const step = event.key === "ArrowDown" ? 1 : -1;
    if (!open.value) {
      if (!props.multiple && list.length) {
        const index = list.findIndex((o) => o.value === props.modelValue);
        const next = list[Math.min(list.length - 1, Math.max(0, index + step))];
        if (next) pick(next);
      } else toggleMenu();
      return;
    }
    hover.value = Math.min(list.length - 1, Math.max(0, hover.value + step));
  } else if (event.key === "Enter" || event.key === " ") {
    event.preventDefault();
    if (!open.value) toggleMenu();
    else if (list[hover.value]) pick(list[hover.value]!);
  }
}
function itemProps(item: Item) {
  return { title: item.title, active: isSelected(props.modelValue, item.value, !!props.multiple), onClick: () => pick(item), value: item.value };
}
function chipProps(item: Item) {
  return { closable: !!props.closableChips, text: item.title, "onClick:close": () => remove(item) };
}

function validate() {
  validated.value = true;
  return !firstRuleError(props.rules, props.modelValue) && messages.value.length === 0;
}
const form = inject(formKey, null);
let unregister: (() => void) | undefined;
onMounted(() => {
  if (props.rules?.length) unregister = form?.register(validate);
});
onBeforeUnmount(() => unregister?.());
defineExpose({ validate, focus: () => field.value?.$el.querySelector<HTMLElement>(".k-select__selection")?.focus() });
</script>

<template>
  <KField
    ref="field"
    class="k-select"
    v-bind="rootAttrs"
    :variant="variant"
    :density="density"
    :label="label"
    :focused="focused || open"
    :floating="floating"
    :error="hasError"
    :disabled="disabled"
    :prepend-inner-icon="prependInnerIcon"
    :show-clear="clearable && hasValue && !disabled"
    :message="message"
    :message-is-error="hasError"
    :hide-details="hideDetails"
    @click.capture="onFieldClick"
    @click:clear="clear"
  >
    <template v-if="$slots['prepend-inner']" #prepend-inner><slot name="prepend-inner" /></template>
    <template #append-inner>
      <KProgressCircular v-if="loading" indeterminate size="18" :width="2" />
      <KIcon v-else icon="mdi-menu-down" />
    </template>
    <div
      class="k-select__selection"
      tabindex="0"
      role="combobox"
      :aria-expanded="open"
      @keydown="onKey"
      @focus="focused = true"
      @blur="onBlur"
    >
      <template v-if="hasValue && !(label && !floating)">
        <template v-if="multiple && chips">
          <template v-for="(item, index) in selected" :key="String(item.value)">
            <slot name="chip" :props="chipProps(item)" :item="item" :index="index">
              <KChip size="small" variant="tonal" :closable="closableChips" @click:close="remove(item)">{{ item.title }}</KChip>
            </slot>
          </template>
        </template>
        <span v-else class="k-select__text">{{ selected.map((s) => s.title).join(", ") }}</span>
      </template>
      <span v-else-if="placeholder && (!label || floating)" class="k-select__placeholder">{{ placeholder }}</span>
    </div>
  </KField>
  <KMenu v-model="open" :anchor="anchorEl" match-width :close-on-content-click="false" :max-height="320" location="bottom start">
    <KList :density="density">
      <template v-for="(item, index) in visibleOptions" :key="String(item.value)">
        <slot name="item" :props="itemProps(item)" :item="item" :index="index">
          <KListItem v-bind="itemProps(item)" :class="{ 'k-list-item--hover': hover === index }" />
        </slot>
      </template>
    </KList>
  </KMenu>
</template>
