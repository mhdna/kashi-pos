<template>
  <div ref="listEl" class="product-list" @mousedown="keepFieldFocus">
    <k-virtual-scroll
      v-if="gridMode"
      ref="scroller"
      class="product-virtual-scroll product-grid-scroll"
      :items="gridRows"
      :item-height="gridRowHeight"
      height="100%"
      @scroll.passive="handleScroll"
    >
      <template #default="{ item: row }">
        <div :key="row[0]?.key" class="product-grid-row" :style="{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }">
          <k-card
            v-for="group in row"
            :key="group.key"
            class="product-cell"
            :class="{ 'product-cell--named': showsName, 'product-cell--group': group.variants.length > 1 }"
            :data-group="group.key"
            variant="flat"
            @click="selectGroup(group)"
          >
            <span v-if="shows('code')" class="product-cell-code"><template
              v-for="(part, i) in hits(group.first.code)"
              :key="i"
            ><span :class="{ 'search-hit': part.hit }">{{ part.text }}</span></template></span>
            <span v-if="showsName" class="product-cell-name text-truncate"><template
              v-for="(part, i) in hits(group.first.name)"
              :key="i"
            ><span :class="{ 'search-hit': part.hit }">{{ part.text }}</span></template></span>
            <span v-if="group.variants.length > 1" class="product-cell-meta">
              <span class="product-cell-variant product-group-options text-medium-emphasis">
                <span v-if="shows('color') && group.colors.length" class="product-group-swatches">
                  <ColorSwatch v-for="c in group.colors.slice(0, MAX_SWATCHES)" :key="c" :name="c" :size="14" />
                  <span v-if="group.colors.length > MAX_SWATCHES" class="product-group-more">+{{ group.colors.length - MAX_SWATCHES }}</span>
                </span>
                <span v-if="shows('size') && group.sizes.length" class="product-group-sizes text-truncate">{{ group.sizes.join(" · ") }}</span>
              </span>
              <span v-if="shows('price')" class="product-price">{{ priceLabel(group.minPrice, group.maxPrice) }}</span>
            </span>
            <span v-else-if="variantLabel(group.first) || shows('price')" class="product-cell-meta">
              <span class="product-cell-variant text-medium-emphasis text-truncate">
                <ColorSwatch v-if="shows('color') && group.first.color" :name="group.first.color" :size="12" class="me-1" />{{ variantLabel(group.first) }}
              </span>
              <span v-if="shows('price')" class="product-price">
                ${{ Number(group.first.price || 0).toFixed(2) }}
                <k-icon
                  v-if="group.first.priceListOverridden"
                  icon="mdi-star"
                  color="amber-darken-2"
                  size="12"
                  :title="group.first.priceListName ? `Price from list: ${group.first.priceListName}` : 'List price'"
                />
              </span>
            </span>
          </k-card>
        </div>
      </template>
    </k-virtual-scroll>
    <k-virtual-scroll
      v-else
      ref="scroller"
      class="product-virtual-scroll"
      :items="groups"
      :item-height="48"
      height="100%"
      @scroll.passive="handleScroll"
    >
      <template #default="{ item: group }">
        <k-list-item
          :key="group.key"
          class="product-list-item"
          :class="{ 'product-list-item--group': group.variants.length > 1 }"
          :data-group="group.key"
          density="comfortable"
          link
          @click="selectGroup(group)"
        >
          <template #title>
            <div class="product-primary-row">
              <span
                v-if="shows('code')"
                class="product-code"
                :class="namesHidden ? 'product-code--lead font-weight-medium' : 'text-medium-emphasis'"
              ><template v-for="(part, i) in hits(group.first.code)" :key="i"><span
                :class="{ 'search-hit': part.hit }"
              >{{ part.text }}</span></template></span>
              <span v-if="showsName" class="font-weight-medium text-truncate"><template
                v-for="(part, i) in hits(group.first.name)"
                :key="i"
              ><span :class="{ 'search-hit': part.hit }">{{ part.text }}</span></template></span>
              <span v-if="group.first.brand && !codeOnly && shows('brand')" class="product-brand text-medium-emphasis">{{ group.first.brand }}</span>
              <template v-if="group.variants.length > 1">
                <span v-if="shows('color') && group.colors.length" class="product-group-swatches">
                  <ColorSwatch v-for="c in group.colors.slice(0, MAX_SWATCHES)" :key="c" :name="c" :size="14" />
                  <span v-if="group.colors.length > MAX_SWATCHES" class="product-group-more text-medium-emphasis">+{{ group.colors.length - MAX_SWATCHES }}</span>
                </span>
                <span v-if="shows('size') && group.sizes.length" class="product-variant product-group-sizes text-medium-emphasis">{{ group.sizes.join(" · ") }}</span>
                <span v-if="shows('price')" class="product-price">{{ priceLabel(group.minPrice, group.maxPrice) }}</span>
              </template>
              <template v-else>
                <span v-if="group.first.color && shows('color')" class="product-variant text-medium-emphasis">
                  <ColorSwatch :name="group.first.color" :size="12" class="me-1" />{{ group.first.color }}
                </span>
                <span v-if="group.first.size && shows('size')" class="product-variant text-medium-emphasis">{{ group.first.size }}</span>
                <span v-if="shows('price')" class="product-price">
                  ${{ Number(group.first.price || 0).toFixed(2) }}
                  <k-icon
                    v-if="group.first.priceListOverridden"
                    icon="mdi-star"
                    color="amber-darken-2"
                    size="12"
                    :title="group.first.priceListName ? `Price from list: ${group.first.priceListName}` : 'List price'"
                  />
                </span>
              </template>
            </div>
          </template>
        </k-list-item>
      </template>
    </k-virtual-scroll>
    <div v-if="loadError" class="product-list-status product-list-status-error">
      <span class="product-error-message text-error">{{ loadErrorMessage }}</span>
      <k-btn size="small" variant="text" @click="loadMore">Retry</k-btn>
    </div>
    <div v-else class="product-list-status text-medium-emphasis">
      <k-progress-circular
        v-if="loading"
        indeterminate
        color="primary"
        size="16"
        width="2"
        class="me-2"
      />
      <span>{{ countLabel }}</span>
    </div>
    <VariantPicker
      v-model="picker.open"
      :code="picker.code"
      :variants="picker.variants"
      refresh
      @select="selectProduct($event as ProductItem)"
    />
  </div>
</template>

<script lang="ts" setup>
import { computed, nextTick, reactive, ref, onBeforeUnmount, watch } from "vue";
import { CountProductGroupsForSearch, ListProductsCursor } from "../../wailsjs/go/main/App";
import { keepFieldFocus } from "@/lib/keepFocus";
import { matchParts } from "@/lib/highlight";
import { groupProducts, priceLabel, type ProductGroup } from "@/lib/productGroups";
import ColorSwatch from "./ColorSwatch.vue";
import VariantPicker from "./VariantPicker.vue";

// The products search's results: a list, or columns of tiles, paged in as
// they're scrolled to. Shown in the search dialog and, in the sale screen's
// own search, in place of the calculator.
//
// The variants sharing a code (one per colour and size) show as one result
// with their colours and sizes; tapping it opens VariantPicker to choose.

export type ProductItem = {
  id: number;
  name: string;
  code: string;
  category: string;
  brand?: string;
  price: number;
  discount: number;
  color: string;
  size: string;
  qty: number;
  priceListOverridden?: boolean;
  priceListName?: string;
  discountListOverridden?: boolean;
  discountListName?: string;
};

const emit = defineEmits<{ select: [product: ProductItem] }>();

const props = withDefaults(
  defineProps<{
    // What's typed; searched as it changes.
    search: string;
    // Searches only while true; becoming true starts afresh.
    active: boolean;
    gridMode: boolean;
    // Hides product names; the backend then matches only code/barcode.
    codeOnly?: boolean;
    // Columns the branch leaves out of the list (central's branch setting).
    hiddenFields?: string[];
  }>(),
  { codeOnly: false, hiddenFields: () => [] },
);
const shows = (field: string) => !props.hiddenFields.includes(field);
// With no name on the row the code is what the cashier reads, so it takes
// the name's place: full size and contrast, and the name's width.
const namesHidden = computed(() => props.codeOnly || !shows("name"));
const showsName = computed(() => !namesHidden.value);

const debouncedSearch = ref("");
let searchTimer: ReturnType<typeof setTimeout> | undefined;
let latestRequest = 0;

// A search answers in a few milliseconds, so it runs as you type; the short
// timer only folds a burst of keys (a paste, a barcode wedge) into one
// request, and latestRequest drops replies a newer search superseded.
watch(
  () => props.search,
  (value) => {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => {
      debouncedSearch.value = value.trim();
    }, 30);
  },
);

watch(debouncedSearch, () => {
  resetList();
});
const batchSize = 50;
const products = ref<ProductItem[]>([]);
const cursor = ref(0);
const scroller = ref<{ scrollToIndex: (index: number) => void; $el?: HTMLElement } | null>(null);
// A new search replaces the list when its first page arrives, rather than
// emptying it first, so typing doesn't flash an empty list on every key.
let replaceOnNextPage = false;
const loading = ref(false);
const hasMore = ref(true);
const loadError = ref(false);
const loadErrorMessage = ref("");
// Full match count for the current search. -1 until the first batch reports
// it; later batches don't recompute it (keeps infinite scroll count-free).
const total = ref(-1);

// The variants loaded so far, one entry per code. A code's later variants
// can arrive with a later page; they join its entry.
const groups = computed(() => groupProducts(products.value));

// Counted in products (codes), as they show, not variants.
const countLabel = computed(() => {
  if (total.value === 0) return "No products found";
  if (total.value < 0) return loading.value ? "Loading…" : "";
  const noun = total.value === 1 ? "product" : "products";
  if (!hasMore.value) return `${total.value} ${noun} · all loaded`;
  return `${total.value} ${noun} · ${groups.value.length} loaded`;
});

function resetList() {
  latestRequest++;
  cursor.value = 0;
  hasMore.value = true;
  replaceOnNextPage = true;
  loadError.value = false;
  loadErrorMessage.value = "";
  // A previous loadMore() may still be in flight and, once superseded, will
  // never get to clear this itself (its finally only fires for the request
  // it still owns) - clear it here so the guard below can't wedge shut.
  loading.value = false;
  if (props.active) {
    void loadMore();
    void loadCount();
  }
}

// The "N results" label, asked beside the first page so the list never
// waits for the count.
let latestCount = 0;
async function loadCount() {
  const request = ++latestCount;
  try {
    const count = await CountProductGroupsForSearch(debouncedSearch.value);
    if (request === latestCount) total.value = Number(count);
  } catch {
    // The label stays as it was; the results don't depend on it.
  }
}

async function loadMore() {
  if (loading.value || !hasMore.value) return;
  const request = ++latestRequest;
  loading.value = true;
  loadError.value = false;
  loadErrorMessage.value = "";
  try {
    const result = await ListProductsCursor(cursor.value, batchSize, debouncedSearch.value);
    if (request !== latestRequest) return;
    const page = result.items.map((product) => ({ ...product, qty: 1 }) as ProductItem);
    if (replaceOnNextPage) {
      replaceOnNextPage = false;
      products.value = page;
      scroller.value?.scrollToIndex(0);
    } else {
      products.value.push(...page);
    }
    cursor.value = result.nextCursor;
    hasMore.value = result.hasMore;
  } catch (e) {
    if (request !== latestRequest) return;
    console.error("ListProductsCursor failed:", e);
    loadError.value = true;
    loadErrorMessage.value = e instanceof Error ? e.message : String(e);
  } finally {
    if (request === latestRequest) loading.value = false;
  }
  if (request === latestRequest && !loadError.value) void nextTick(fillScreen);
}

// A page in columns can be shorter than the list, leaving nothing to scroll
// that would ask for the next one; keep asking until the list is full.
// Measured from the rows, not the scroller, which grows a frame later.
function fillScreen() {
  const element = scroller.value?.$el;
  if (!element) return;
  const height = props.gridMode ? gridRows.value.length * gridRowHeight.value : groups.value.length * 48;
  if (height - element.clientHeight <= 120) void loadMore();
}

function handleScroll(event: Event) {
  const element = event.currentTarget as HTMLElement;
  if (element.scrollHeight - element.scrollTop - element.clientHeight <= 120) void loadMore();
}

function selectProduct(product: ProductItem) {
  emit("select", { ...product, qty: 1 });
}

// One variant goes straight in; several open the picker, which loads the
// code's whole set (this list may hold only some of them yet).
const MAX_SWATCHES = 6;
const picker = reactive<{ open: boolean; code: string; variants: ProductItem[] }>({ open: false, code: "", variants: [] });
function selectGroup(group: ProductGroup<ProductItem>) {
  if (group.variants.length === 1) {
    selectProduct(group.first);
    return;
  }
  picker.code = group.first.code;
  picker.variants = group.variants;
  picker.open = true;
}

onBeforeUnmount(() => {
  clearTimeout(searchTimer);
  latestRequest++;
});

// What was searched for, coloured wherever it shows in a result's code or
// name. The term the shown results were found with, not the one being typed.
const hits = (text: string) => matchParts(text || "", debouncedSearch.value);
const variantLabel = (product: ProductItem) =>
  [shows("color") && product.color, shows("size") && product.size].filter(Boolean).join(" · ");

const listEl = ref<HTMLElement | null>(null);
const listWidth = ref(0);
const resizer = new ResizeObserver(([entry]) => {
  listWidth.value = entry?.contentRect.width ?? 0;
});
watch(listEl, (el, old) => {
  if (old) resizer.unobserve(old);
  if (el) resizer.observe(el);
});
// Four to a row at most, so each code stays big enough to read at a glance;
// fewer where the window is too narrow for four.
const MAX_COLUMNS = 4;
const columns = computed(() => Math.min(MAX_COLUMNS, Math.max(1, Math.floor((listWidth.value - 24) / 200))));
const gridRowHeight = computed(() => (showsName.value ? 96 : 68));
const gridRows = computed(() => {
  const rows: ProductGroup<ProductItem>[][] = [];
  for (let i = 0; i < groups.value.length; i += columns.value) rows.push(groups.value.slice(i, i + columns.value));
  return rows;
});
watch([() => props.gridMode, columns], () => void nextTick(fillScreen));

// Declared last: it runs at once, and needs everything above.
watch(
  () => props.active,
  (active) => {
    clearTimeout(searchTimer);
    if (!active) return;
    debouncedSearch.value = props.search.trim();
    resetList();
  },
  { immediate: true },
);

onBeforeUnmount(() => resizer.disconnect());
</script>

<style scoped>
/* Like a highlighter pen: yellow behind the match, the text kept dark. */
.search-hit {
  background-color: #ffe066;
  color: #1f1f1f;
  border-radius: 3px;
  box-decoration-break: clone;
}
/* In the dark theme the pen is a soft amber wash with the light text kept, not
   a bright block with light text on it. */
:global(.k-theme--dark) .search-hit {
  background-color: rgba(255, 214, 64, 0.32);
  color: rgb(var(--k-theme-on-surface));
}

.product-grid-row {
  display: grid;
  gap: 8px;
  height: 100%;
  padding: 4px 12px;
}

.product-cell {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 1px;
  min-width: 0;
  padding: 6px 14px;
  border-radius: 10px;
  background: rgba(var(--k-theme-on-surface), 0.045);
}

.product-cell-code {
  font-size: 1.3rem;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.product-cell-name {
  font-size: 1.05rem;
}

.product-cell--named .product-cell-code {
  font-size: 0.95rem;
  color: rgba(var(--k-theme-on-surface), var(--k-medium-emphasis-opacity));
}

.product-cell--named .product-cell-code .search-hit {
  color: #1f1f1f;
}
:global(.k-theme--dark) .product-cell--named .product-cell-code .search-hit {
  color: rgb(var(--k-theme-on-surface));
}

.product-cell-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  font-size: 0.9rem;
}

.product-cell-variant {
  flex: 1 1 auto;
  min-width: 0;
}


.product-list {
  height: 100%;
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.product-virtual-scroll {
  flex: 1 1 auto;
  min-height: 0;
}

.product-list-item {
  border-bottom: thin solid rgba(var(--k-border-color), var(--k-border-opacity));
}

.product-primary-row {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.product-primary-row > :nth-child(2) {
  flex: 1 1 auto;
}

.product-price {
  flex: 0 0 auto;
  font-weight: 600;
}

.product-code,
.product-brand,
.product-group-options {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.product-group-swatches {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  flex: none;
}
.product-group-more {
  font-size: 0.75rem;
  margin-left: 2px;
}
.product-group-sizes {
  min-width: 0;
}
.product-variant {
  font-size: 0.75rem;
}

.product-code {
  flex: 0 0 96px;
  width: 96px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.product-code--lead {
  flex: 1 1 auto;
  width: auto;
  min-width: 0;
  font-size: 1rem;
}

.product-brand {
  flex: 0 0 96px;
  width: 96px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.product-list-status {
  flex: 0 0 auto;
  min-height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.8125rem;
}

.product-list-status-error {
  flex-direction: column;
  gap: 2px;
  padding: 8px 16px;
}

.product-error-message {
  font-size: 0.75rem;
  text-align: center;
  word-break: break-word;
}
</style>
