<script setup lang="ts">
import { computed, onBeforeUnmount, ref, useAttrs, watch } from "vue";
import KField from "./KField.vue";
import KList from "./KList.vue";
import KListItem from "./KListItem.vue";
import KMenu from "./KMenu.vue";
import KProgressCircular from "./KProgressCircular.vue";
import { normaliseItems, type Item } from "./items";

defineOptions({ inheritAttrs: false });
const props = defineProps<{
  modelValue?: unknown;
  search?: string;
  items?: readonly unknown[];
  itemTitle?: string | ((item: any) => unknown);
  itemValue?: string | ((item: any) => unknown);
  noFilter?: boolean;
  loading?: boolean;
  placeholder?: string;
  label?: string;
  variant?: string;
  density?: string;
  hideDetails?: boolean | string;
  disabled?: boolean;
  clearable?: boolean;
}>();
const emit = defineEmits<{ "update:modelValue": [value: any]; "update:search": [value: string] }>();

const attrs = useAttrs();
const rootAttrs = computed(() => ({ class: attrs.class, style: attrs.style }));
const field = ref<{ $el: HTMLElement } | null>(null);
const anchorEl = ref<HTMLElement | null>(null);
const open = ref(false);
const focused = ref(false);
const typed = ref<string | null>(null); // text the user typed since the last pick

const options = computed(() => normaliseItems(props.items, props.itemTitle, props.itemValue));
const selectedTitle = computed(() => options.value.find((o) => o.value === props.modelValue)?.title ?? "");
const shown = computed(() => typed.value ?? props.search ?? selectedTitle.value);
const filtered = computed(() => {
  if (props.noFilter || typed.value === null) return options.value;
  const q = typed.value.toLowerCase();
  return options.value.filter((o) => o.title.toLowerCase().includes(q));
});

function openMenu() {
  anchorEl.value = field.value?.$el.querySelector<HTMLElement>(".k-field") ?? null;
  open.value = true;
}
function onInput(event: Event) {
  const value = (event.target as HTMLInputElement).value;
  typed.value = value;
  emit("update:search", value);
  openMenu();
}
function pick(item: Item) {
  typed.value = null;
  emit("update:modelValue", item.value);
  emit("update:search", item.title);
  open.value = false;
}
function onBlur() {
  focused.value = false;
  typed.value = null;
}
watch(
  () => props.modelValue,
  () => (typed.value = null),
);
const input = ref<HTMLInputElement | null>(null);
onBeforeUnmount(() => (open.value = false));
defineExpose({ focus: () => input.value?.focus() });
</script>

<template>
  <KField
    ref="field"
    v-bind="rootAttrs"
    :variant="variant"
    :density="density"
    :label="label"
    :focused="focused"
    :floating="focused || !!shown"
    :disabled="disabled"
    :hide-details="hideDetails"
  >
    <template #append-inner><KProgressCircular v-if="loading" indeterminate size="18" :width="2" /></template>
    <input
      ref="input"
      v-bind="Object.fromEntries(Object.entries(attrs).filter(([k]) => k !== 'class' && k !== 'style'))"
      class="k-field__input"
      type="text"
      autocomplete="off"
      :value="shown"
      :placeholder="placeholder"
      :disabled="disabled"
      @input="onInput"
      @focus="((focused = true), openMenu())"
      @blur="onBlur"
    />
  </KField>
  <KMenu v-model="open" :anchor="anchorEl" match-width :close-on-content-click="false" :max-height="320">
    <KList>
      <KListItem v-for="item in filtered" :key="String(item.value)" :title="item.title" :active="item.value === modelValue" @mousedown.prevent @click="pick(item)" />
      <KListItem v-if="!filtered.length && !loading" title="No data available" />
    </KList>
  </KMenu>
</template>
