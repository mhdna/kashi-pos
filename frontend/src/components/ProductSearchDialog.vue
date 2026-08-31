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
      <div :key="listKey" class="product-list">
        <v-virtual-scroll
          class="product-virtual-scroll"
          :items="products"
          :item-height="44"
          height="360"
          @scroll.passive="handleScroll"
        >
          <template #default="{ item: product }">
            <v-list-item
              :key="product.id"
              class="product-list-item"
              density="compact"
              link
              @click="selectProduct(product)"
            >
              <template #title>
                <div class="product-primary-row">
                  <span class="product-code text-medium-emphasis">{{ product.code }}</span>
                  <span class="font-weight-medium text-truncate">{{ product.name }}</span>
                  <span v-if="product.color" class="product-variant text-medium-emphasis">{{ product.color }}</span>
                  <span v-if="product.size" class="product-variant text-medium-emphasis">{{ product.size }}</span>
                  <span class="product-price">
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
        <div v-if="loading" class="product-list-status">
          <v-progress-circular indeterminate color="primary" size="24" width="2" />
        </div>
        <div v-else-if="loadError" class="product-list-status product-list-status-error">
          <span class="product-error-message text-error">{{ loadErrorMessage }}</span>
          <v-btn size="small" variant="text" @click="loadMore">Retry</v-btn>
        </div>
        <div v-else-if="!hasMore" class="product-list-status text-medium-emphasis">
          {{ products.length ? "All products loaded" : "No products found" }}
        </div>
      </div>
      <v-divider />
      <div class="product-search-row">
        <v-text-field
          v-model="search"
          class="product-search-control"
          density="compact"
          hide-details
          placeholder="Search products..."
          variant="outlined"
          autofocus
        />
      </div>
      <v-card-text class="product-inline-keyboard">
        <InlineKeyboard v-model="search" :max-length="80" layout="code" />
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
import { ref, inject, onBeforeUnmount, watch } from "vue";
import { ListProductsCursor } from "../../wailsjs/go/main/App";
import InlineKeyboard from "@/components/InlineKeyboard.vue";

type ProductItem = {
  id: number;
  name: string;
  code: string;
  category: string;
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
  }>(),
  {
    activatorText: "Products List",
    activatorIcon: "mdi-format-list-bulleted-square",
    showActivator: true,
  },
);
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
  if (v) resetList();
});

watch(search, (value) => {
  clearTimeout(searchTimer);
  latestRequest++;
  searchTimer = setTimeout(() => {
    debouncedSearch.value = value.trim();
  }, 250);
});

watch(debouncedSearch, () => {
  resetList();
});

const batchSize = 50;
const products = ref<ProductItem[]>([]);
const cursor = ref(0);
const listKey = ref(0);
const loading = ref(false);
const hasMore = ref(true);
const loadError = ref(false);
const loadErrorMessage = ref("");

function resetList() {
  latestRequest++;
  products.value = [];
  cursor.value = 0;
  hasMore.value = true;
  loadError.value = false;
  loadErrorMessage.value = "";
  // A previous loadMore() may still be in flight and, once superseded, will
  // never get to clear this itself (its finally only fires for the request
  // it still owns) - clear it here so the guard below can't wedge shut.
  loading.value = false;
  listKey.value++;
  if (dialog.value) void loadMore();
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
    products.value.push(...result.items.map((product) => ({ ...product, qty: 1 }) as ProductItem));
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

defineExpose({ openDialog });
</script>

<style scoped>
.product-search-control {
  width: 100%;
}

.product-search-row {
  padding: 12px 16px;
}

.product-list-card {
  height: 100%;
  position: relative;
  display: flex;
  flex-direction: column;
}

.product-list {
  flex: 0 0 auto;
  min-height: 0;
}

.product-virtual-scroll {
  height: 360px;
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

.product-list-status {
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

.product-inline-keyboard {
  flex: 0 0 205px;
  height: 205px;
  display: flex;
  align-items: center;
  margin-top: auto;
  margin-bottom: 48px;
  margin-inline: auto;
  max-width: 600px;
  padding-top: 12px;
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
