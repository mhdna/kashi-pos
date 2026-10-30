<template>
  <v-dialog
    v-model="dialog"
    class="product-search-dialog"
    max-width="none"
    width="auto"
  >
    <template v-if="showActivator" #activator="{ props: activatorProps }">
      <v-btn
        v-bind="activatorProps"
        :icon="!activatorText"
        :aria-label="activatorText || 'Add products'"
        variant="text"
        class="ms-1"
      >
        <v-icon :icon="activatorIcon" size="20" :class="{ 'me-1': Boolean(activatorText) }" />
        <template v-if="activatorText">{{ activatorText }}</template>
      </v-btn>
    </template>
    <v-card class="product-list-card">
      <div class="product-list">
        <v-virtual-scroll
          ref="scroller"
          class="product-virtual-scroll"
          :items="products"
          :item-height="48"
          height="100%"
          @scroll.passive="handleScroll"
        >
          <template #default="{ item: product }">
            <v-list-item
              :key="product.id"
              class="product-list-item"
              density="comfortable"
              link
              @click="selectProduct(product)"
            >
              <template #title>
                <div class="product-primary-row">
                  <span
                    v-if="shows('code')"
                    class="product-code"
                    :class="namesHidden ? 'product-code--lead font-weight-medium' : 'text-medium-emphasis'"
                  >{{ product.code }}</span>
                  <span v-if="!codeOnly && shows('name')" class="font-weight-medium text-truncate">{{ product.name }}</span>
                  <span v-if="product.brand && !codeOnly && shows('brand')" class="product-brand text-medium-emphasis">{{ product.brand }}</span>
                  <span v-if="product.color && shows('color')" class="product-variant text-medium-emphasis">{{ product.color }}</span>
                  <span v-if="product.size && shows('size')" class="product-variant text-medium-emphasis">{{ product.size }}</span>
                  <span v-if="shows('price')" class="product-price">
                    ${{ Number(product.price || 0).toFixed(2) }}
                    <v-icon
                      v-if="product.priceListOverridden"
                      icon="mdi-star"
                      color="amber-darken-2"
                      size="12"
                      :title="product.priceListName ? `Price from list: ${product.priceListName}` : 'List price'"
                    />
                  </span>
                </div>
              </template>
            </v-list-item>
          </template>
        </v-virtual-scroll>
        <div v-if="loadError" class="product-list-status product-list-status-error">
          <span class="product-error-message text-error">{{ loadErrorMessage }}</span>
          <v-btn size="small" variant="text" @click="loadMore">Retry</v-btn>
        </div>
        <div v-else class="product-list-status text-medium-emphasis">
          <v-progress-circular
            v-if="loading"
            indeterminate
            color="primary"
            size="16"
            width="2"
            class="me-2"
          />
          <span>{{ countLabel }}</span>
        </div>
      </div>
      <v-divider />
      <div class="product-search-row">
        <v-text-field
          ref="searchField"
          v-model="search"
          class="product-search-control"
          density="compact"
          hide-details
          inputmode="none"
          :placeholder="codeOnly ? 'Enter a product code...' : 'Search products...'"
          variant="outlined"
          autofocus
        />
      </div>
      <v-card-text class="product-inline-keyboard">
        <InlineKeyboard v-model="search" :input="searchInput" :max-length="80" layout="code" />
      </v-card-text>
      <v-btn
        class="product-dialog-close"
        color="error"
        variant="tonal"
        aria-label="Close product search"
        @click="dialog = false"
      >
        <v-icon icon="mdi-close" size="28" class="me-1" />
        Close
      </v-btn>
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
import { computed, ref, inject, onBeforeUnmount, watch } from "vue";
import { CountProductsForSearch, ListProductsCursor } from "../../wailsjs/go/main/App";
import InlineKeyboard from "@/components/InlineKeyboard.vue";
import { useTouchSelection } from "@/composables/useTouchSelection";

type ProductItem = {
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

const emit = defineEmits<{
  select: [
    product: {
      name: string;
      code: string;
      category: string;
      price: number;
      discount: number;
      qty: number;
    },
  ];
}>();

const props = withDefaults(
  defineProps<{
    activatorText?: string;
    activatorIcon?: string;
    showActivator?: boolean;
    // Hides product names; the backend then matches only code/barcode.
    codeOnly?: boolean;
    // Columns the branch leaves out of the list (kashi's branch setting).
    hiddenFields?: string[];
  }>(),
  {
    activatorText: "Products List",
    activatorIcon: "mdi-format-list-bulleted-square",
    showActivator: true,
    codeOnly: false,
    hiddenFields: () => [],
  },
);
const shows = (field: string) => !props.hiddenFields.includes(field);
// With no name on the row the code is what the cashier reads, so it takes
// the name's place: full size and contrast, and the name's width.
const namesHidden = computed(() => props.codeOnly || !shows("name"));
const activatorText = props.activatorText;
const activatorIcon = props.activatorIcon;
const showActivator = props.showActivator;

const dialogOpen = inject<import("vue").Ref<boolean>>("dialogOpen", { value: false } as any);
const dialog = ref(false);
const search = ref("");
const debouncedSearch = ref("");
let searchTimer: ReturnType<typeof setTimeout> | undefined;
let latestRequest = 0;

watch(dialog, (v) => {
  dialogOpen.value = v;
  if (v) {
    resetList();
  } else {
    // Don't let the previous query (and its results) linger for the next open.
    clearTimeout(searchTimer);
    search.value = "";
    debouncedSearch.value = "";
  }
});

// A search answers in a few milliseconds, so it runs as you type; the short
// timer only folds a burst of keys (a paste, a barcode wedge) into one
// request, and latestRequest drops replies a newer search superseded.
watch(search, (value) => {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => {
    debouncedSearch.value = value.trim();
  }, 30);
});

watch(debouncedSearch, () => {
  resetList();
});

const batchSize = 50;
const products = ref<ProductItem[]>([]);
const cursor = ref(0);
const scroller = ref<{ scrollToIndex: (index: number) => void } | null>(null);
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

const countLabel = computed(() => {
  if (total.value === 0) return "No products found";
  if (total.value < 0) return loading.value ? "Loading…" : "";
  const noun = total.value === 1 ? "result" : "results";
  if (!hasMore.value) return `${total.value} ${noun} · all loaded`;
  return `${total.value} ${noun} · ${products.value.length} loaded`;
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
  if (dialog.value) {
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
    const count = await CountProductsForSearch(debouncedSearch.value);
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
}

function handleScroll(event: Event) {
  const element = event.currentTarget as HTMLElement;
  if (element.scrollHeight - element.scrollTop - element.clientHeight <= 120) void loadMore();
}

function selectProduct(product: ProductItem) {
  emit("select", product);
  dialog.value = false;
  search.value = "";
}

onBeforeUnmount(() => {
  clearTimeout(searchTimer);
  latestRequest++;
});

function openDialog() {
  dialog.value = true;
}

// The search field's input, for the keyboard's caret and touch selection.
const searchField = ref<{ $el?: HTMLElement } | null>(null);
const searchInput = ref<HTMLInputElement | null>(null);
watch(searchField, (field) => {
  searchInput.value = field?.$el?.querySelector("input") ?? null;
}, { flush: "post" });
useTouchSelection(searchInput);

defineExpose({ openDialog });
</script>

<style scoped>
.product-search-control {
  width: 100%;
}

.product-search-row {
  flex: 0 0 auto;
  padding: 8px 16px 6px;
}

.product-list-card {
  height: 100%;
  position: relative;
  display: flex;
  flex-direction: column;
}

/* Fills the space left above the search bar + keyboard so the search bar
   stays anchored and never shifts as the result count changes. */
.product-list {
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
  border-bottom: thin solid rgba(var(--v-border-color), var(--v-border-opacity));
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

/* Must be tall enough for the full "code" layout keyboard (number row + 3
   letter rows + actions row = ~264px). If this is shorter the keyboard
   overflows upward and covers the search bar above it. */
.product-inline-keyboard {
  flex: 0 0 264px;
  height: 264px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  justify-content: flex-start;
  margin-bottom: 48px;
  margin-inline: auto;
  /* The keys flex to share whatever width this element ends up with, so they
     can't overflow; this cap just stops them looking oversized on a wide
     screen. overflow: hidden above only guards against vertical spill. */
  width: 100%;
  max-width: 748px;
  padding-inline: 8px;
  padding-top: 4px;
  padding-bottom: 0;
}

.product-dialog-close {
  position: absolute;
  right: 16px;
  bottom: 16px;
  height: 48px;
  z-index: 1;
}

:global(.product-search-dialog .v-overlay__content) {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  left: 72px;
  width: auto !important;
  height: 100vh !important;
  min-height: 100vh !important;
  max-width: none !important;
  max-height: none !important;
  margin: 0;
}

:global(.kashi-compact .product-search-dialog .v-overlay__content) {
  left: 80px;
}
</style>
