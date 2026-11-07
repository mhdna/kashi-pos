<script setup lang="ts">
// Choose a colour and a size of a product code sold in several variants.
//
// It opens with the variants the caller already has and, when `refresh` is
// set, swaps in the whole set from the backend (the search may have loaded
// only some). A colour or size no variant has with the other choice is
// greyed out; a choice with a single option is made for the cashier. When
// the choices leave several variants (same colour and size twice, or
// variants told apart by neither), they are listed to pick from.
import { computed, ref, watch } from "vue";
import { ListProductVariants } from "../../wailsjs/go/main/App";
import ColorSwatch from "./ColorSwatch.vue";
import { colorAvailable, matching, priceLabel, sizeAvailable, summarise, type VariantLike } from "@/lib/productGroups";

type Variant = VariantLike & Record<string, any>;

const props = withDefaults(
  defineProps<{
    modelValue: boolean;
    code: string;
    variants: Variant[];
    refresh?: boolean;
  }>(),
  { refresh: false },
);
const emit = defineEmits<{ "update:modelValue": [value: boolean]; select: [variant: Variant] }>();

const all = ref<Variant[]>([]);
const color = ref<string | null>(null);
const size = ref<string | null>(null);
const loading = ref(false);
let request = 0;

const group = computed(() => (all.value.length ? summarise(props.code, all.value) : null));
const colors = computed(() => group.value?.colors ?? []);
const sizes = computed(() => group.value?.sizes ?? []);
const name = computed(() => group.value?.first.name ?? "");
const choices = computed(() => matching(all.value, color.value, size.value));
// Every offered dimension has a choice.
const chosen = computed(() => (!colors.value.length || color.value !== null) && (!sizes.value.length || size.value !== null));
const picked = computed(() => (chosen.value && choices.value.length === 1 ? choices.value[0]! : null));
// Several variants left once everything is chosen: list them.
const listed = computed(() => (chosen.value && choices.value.length > 1 ? choices.value : []));
const footerPrice = computed(() => {
  const prices = choices.value.map((v) => Number(v.price || 0));
  return prices.length ? priceLabel(Math.min(...prices), Math.max(...prices)) : "";
});

function autoPick() {
  if (colors.value.length === 1) color.value = colors.value[0]!;
  if (sizes.value.length === 1) size.value = sizes.value[0]!;
}

function keepValid() {
  if (color.value !== null && !colors.value.some((c) => same(c, color.value))) color.value = null;
  if (size.value !== null && !sizes.value.some((s) => same(s, size.value))) size.value = null;
  autoPick();
}

const same = (a: string, b: string | null) => a.toLowerCase() === (b ?? "").toLowerCase();

async function loadAll() {
  const mine = ++request;
  loading.value = true;
  try {
    const fresh = ((await ListProductVariants(props.code)) ?? []) as Variant[];
    if (mine !== request || !props.modelValue || !fresh.length) return;
    all.value = fresh.map((v) => ({ ...v, qty: 1 }));
    keepValid();
  } catch {
    // Keep what the caller gave; it is still a valid set to choose from.
  } finally {
    if (mine === request) loading.value = false;
  }
}

watch(
  () => props.modelValue,
  (open) => {
    request++;
    loading.value = false;
    if (!open) return;
    all.value = props.variants.map((v) => ({ ...v }));
    color.value = null;
    size.value = null;
    autoPick();
    if (props.refresh && props.code) void loadAll();
  },
  { immediate: true },
);

function toggleColor(c: string) {
  if (!colorAvailable(all.value, c, size.value) && size.value !== null) size.value = null;
  color.value = same(c, color.value) ? (colors.value.length === 1 ? color.value : null) : c;
}
function toggleSize(s: string) {
  if (!sizeAvailable(all.value, s, color.value) && color.value !== null) color.value = null;
  size.value = same(s, size.value) ? (sizes.value.length === 1 ? size.value : null) : s;
}

function close() {
  emit("update:modelValue", false);
}
function add(variant: Variant | null) {
  if (!variant) return;
  emit("select", variant);
  close();
}
</script>

<template>
  <k-dialog :model-value="modelValue" max-width="620" scrollable @update:model-value="emit('update:modelValue', $event)">
    <k-card class="variant-picker">
      <k-card-title class="variant-picker-title">
        <span class="variant-picker-code">{{ code }}</span>
        <k-progress-circular v-if="loading" indeterminate size="16" :width="2" class="ms-2" />
      </k-card-title>
      <div v-if="name" class="variant-picker-name text-medium-emphasis" dir="auto">{{ name }}</div>
      <k-card-text class="variant-picker-body">
        <section v-if="colors.length" class="variant-picker-section">
          <div class="variant-picker-label">Color</div>
          <div class="variant-picker-options">
            <button
              v-for="c in colors"
              :key="c"
              type="button"
              class="variant-option variant-option--color"
              :class="{ 'variant-option--selected': same(c, color) }"
              :aria-pressed="same(c, color)"
              :data-unavailable="!colorAvailable(all, c, size) || undefined"
              @click="toggleColor(c)"
            >
              <ColorSwatch :name="c" :size="26" />
              <span class="variant-option-text" dir="auto">{{ c }}</span>
            </button>
          </div>
        </section>
        <section v-if="sizes.length" class="variant-picker-section">
          <div class="variant-picker-label">Size</div>
          <div class="variant-picker-options">
            <button
              v-for="s in sizes"
              :key="s"
              type="button"
              class="variant-option variant-option--size"
              :class="{ 'variant-option--selected': same(s, size) }"
              :aria-pressed="same(s, size)"
              :data-unavailable="!sizeAvailable(all, s, color) || undefined"
              @click="toggleSize(s)"
            >
              <span class="variant-option-text">{{ s }}</span>
            </button>
          </div>
        </section>
        <section v-if="listed.length || (!colors.length && !sizes.length)" class="variant-picker-section">
          <div class="variant-picker-label">{{ listed.length ? `${listed.length} matching items` : "Items" }}</div>
          <k-list class="variant-picker-list">
            <k-list-item
              v-for="v in listed.length ? listed : all"
              :key="v.id"
              link
              class="variant-picker-item"
              :title="v.name"
              :subtitle="v.barcode"
              @click="add(v)"
            >
              <template #append>
                <span class="font-weight-medium">${{ Number(v.price || 0).toFixed(2) }}</span>
              </template>
            </k-list-item>
          </k-list>
        </section>
      </k-card-text>
      <k-card-actions class="variant-picker-actions">
        <span class="variant-picker-summary">
          <template v-if="picked">
            <ColorSwatch v-if="picked.color" :name="picked.color" :size="18" />
            <span dir="auto">{{ [picked.color, picked.size].filter(Boolean).join(" · ") }}</span>
          </template>
          <span v-else-if="!listed.length" class="text-medium-emphasis">
            {{ colors.length && color === null ? "Choose a color" : sizes.length && size === null ? "Choose a size" : "" }}
          </span>
          <span v-if="footerPrice" class="variant-picker-price">{{ footerPrice }}</span>
        </span>
        <k-spacer />
        <k-btn variant="text" class="variant-picker-cancel" @click="close">Cancel</k-btn>
        <k-btn color="primary" variant="flat" class="variant-picker-add" :disabled="!picked" @click="add(picked)">Add</k-btn>
      </k-card-actions>
    </k-card>
  </k-dialog>
</template>

<style scoped>
.variant-picker-title { display: flex; align-items: center; padding-bottom: 0; }
.variant-picker-code { font-size: 1.35rem; font-weight: 700; letter-spacing: 0.02em; }
.variant-picker-name { padding: 0 24px; font-size: 0.95rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.variant-picker-body { display: flex; flex-direction: column; gap: 16px; padding-top: 16px; }
.variant-picker-label { font-size: 0.8rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.06em; margin-bottom: 8px; color: rgba(var(--k-theme-on-surface), var(--k-medium-emphasis-opacity)); }
.variant-picker-options { display: flex; flex-wrap: wrap; gap: 8px; }
.variant-option {
  display: inline-flex; align-items: center; gap: 8px;
  min-height: 48px; min-width: 64px; padding: 6px 14px;
  border-radius: 10px; border: 2px solid rgba(var(--k-theme-on-surface), 0.14);
  background: rgb(var(--k-theme-surface)); color: rgb(var(--k-theme-on-surface));
  font: inherit; font-size: 1rem; font-weight: 500; cursor: pointer;
  justify-content: center;
}
.variant-option--size { min-width: 72px; }
.variant-option--selected { border-color: rgb(var(--k-theme-primary)); background: rgba(var(--k-theme-primary), 0.1); }
.variant-option[data-unavailable] { opacity: 0.38; text-decoration: line-through; }
.variant-option-text { white-space: nowrap; }
.variant-picker-actions { gap: 8px; padding: 12px 16px; border-top: thin solid rgba(var(--k-border-color), var(--k-border-opacity)); }
.variant-picker-summary { display: inline-flex; align-items: center; gap: 8px; min-width: 0; font-weight: 500; }
.variant-picker-price { font-weight: 700; font-size: 1.1rem; }
.variant-picker-add { min-width: 96px; }
</style>
