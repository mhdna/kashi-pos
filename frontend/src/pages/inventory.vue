<template>
  <!-- Same list as the product search dialog, as a page: products scroll in
       batches above a search bar and the on-screen keyboard, and each row
       carries what the other branches hold of it. -->
  <div class="inventory-page">
    <div :key="listKey" class="inventory-list">
      <k-virtual-scroll
        class="inventory-virtual-scroll"
        :items="products"
        :item-height="52"
        height="100%"
        @scroll.passive="handleScroll"
      >
        <template #default="{ item: product }">
          <k-list-item
            :key="product.id"
            class="inventory-list-item"
            density="comfortable"
            link
            @click="openProduct(product)"
          >
            <template #title>
              <div class="inventory-row">
                <span class="inventory-code text-medium-emphasis">{{ product.code }}</span>
                <span class="inventory-name font-weight-medium text-truncate">{{ product.name }}</span>
                <span v-if="product.brand" class="inventory-meta text-medium-emphasis">{{ product.brand }}</span>
                <span v-if="product.color" class="inventory-variant text-medium-emphasis">{{ product.color }}</span>
                <span v-if="product.size" class="inventory-variant text-medium-emphasis">{{ product.size }}</span>
                <span class="inventory-branches">
                  <k-chip
                    v-for="branch in product.branches.slice(0, 4)"
                    :key="branch.branchCode"
                    :color="branch.quantity > 0 ? 'primary' : undefined"
                    size="x-small"
                    variant="tonal"
                  >
                    {{ branch.branchCode }} {{ branch.quantity }}
                  </k-chip>
                  <span v-if="product.branches.length > 4" class="text-caption text-medium-emphasis">
                    +{{ product.branches.length - 4 }}
                  </span>
                </span>
                <span class="inventory-price">${{ Number(product.price || 0).toFixed(2) }}</span>
              </div>
            </template>
          </k-list-item>
        </template>
      </k-virtual-scroll>
      <div v-if="loadError" class="inventory-status inventory-status-error">
        <span class="text-error text-caption">{{ loadErrorMessage }}</span>
        <k-btn size="small" variant="text" @click="loadMore">Retry</k-btn>
      </div>
      <div v-else class="inventory-status text-medium-emphasis">
        <k-progress-circular v-if="loading" indeterminate color="primary" size="16" width="2" class="me-2" />
        <span>{{ countLabel }}</span>
      </div>
    </div>
    <k-divider />
    <div class="inventory-search-row">
      <k-text-field
        v-model="search"
        density="compact"
        hide-details
        placeholder="Search by name, code or barcode..."
        prepend-inner-icon="mdi-magnify"
        variant="outlined"
        clearable
        autofocus
      />
    </div>
    <div class="inventory-keyboard">
      <InlineKeyboard v-model="search" :max-length="80" layout="code" />
    </div>
  </div>

  <k-dialog v-model="detailOpen" max-width="560">
    <k-card v-if="selected">
      <k-card-title class="d-flex align-center ga-2">
        <span class="text-truncate">{{ selected.name }}</span>
        <k-spacer />
        <k-btn icon="mdi-close" variant="text" density="comfortable" @click="detailOpen = false" />
      </k-card-title>
      <k-card-subtitle>
        {{ [selected.code, selected.barcode, selected.brand, selected.color, selected.size].filter(Boolean).join(" · ") }}
      </k-card-subtitle>
      <k-card-text>
        <div class="d-flex justify-space-between font-weight-bold mb-3">
          <span>Price here</span>
          <span>${{ Number(selected.price || 0).toFixed(2) }}</span>
        </div>
        <k-table v-if="selected.branches.length" density="compact">
          <thead>
            <tr>
              <th>Branch</th>
              <th class="text-end">On hand</th>
              <th class="text-end">Their price</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="branch in selected.branches" :key="branch.branchCode">
              <td>{{ branch.branchCode }}</td>
              <td class="text-end">{{ branch.quantity }}</td>
              <td class="text-end">${{ (branch.priceCents / 100).toFixed(2) }}</td>
            </tr>
          </tbody>
        </k-table>
        <div v-else class="text-medium-emphasis text-center py-4">
          No other branch has stock of this product.
        </div>
      </k-card-text>
    </k-card>
  </k-dialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { ListInventoryCursor } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import InlineKeyboard from "@/components/InlineKeyboard.vue";

type BranchStock = { branchCode: string; quantity: number; priceCents: number };
type InventoryItem = {
  id: number;
  name: string;
  code: string;
  barcode: string;
  brand?: string;
  color: string;
  size: string;
  price: number;
  branches: BranchStock[];
  otherStock: number;
};

const search = ref("");
const debouncedSearch = ref("");
let searchTimer: ReturnType<typeof setTimeout> | undefined;

watch(search, (value) => {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => {
    debouncedSearch.value = (value ?? "").trim();
  }, 250);
});
watch(debouncedSearch, () => resetList());

// Same cursor paging as ProductSearchDialog.
const batchSize = 50;
const products = ref<InventoryItem[]>([]);
const cursor = ref(0);
const listKey = ref(0);
const loading = ref(false);
const hasMore = ref(true);
const loadError = ref(false);
const loadErrorMessage = ref("");
const total = ref(-1);
let latestRequest = 0;

const countLabel = computed(() => {
  if (total.value === 0) return "No products found";
  if (total.value < 0) return loading.value ? "Loading…" : "";
  const noun = total.value === 1 ? "product" : "products";
  if (!hasMore.value) return `${total.value} ${noun} · all loaded`;
  return `${total.value} ${noun} · ${products.value.length} loaded`;
});

function resetList() {
  latestRequest++;
  products.value = [];
  cursor.value = 0;
  hasMore.value = true;
  total.value = -1;
  loadError.value = false;
  loadErrorMessage.value = "";
  loading.value = false;
  listKey.value++;
  void loadMore();
}

async function loadMore() {
  if (loading.value || !hasMore.value) return;
  const request = ++latestRequest;
  loading.value = true;
  loadError.value = false;
  loadErrorMessage.value = "";
  try {
    const result = await ListInventoryCursor(cursor.value, batchSize, debouncedSearch.value);
    if (request !== latestRequest) return;
    products.value.push(...(result.items as InventoryItem[]));
    cursor.value = result.nextCursor;
    hasMore.value = result.hasMore;
    if (typeof result.total === "number" && result.total >= 0) total.value = result.total;
  } catch (e) {
    if (request !== latestRequest) return;
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

const detailOpen = ref(false);
const selected = ref<InventoryItem | null>(null);
function openProduct(product: InventoryItem) {
  selected.value = product;
  detailOpen.value = true;
}

// Other branches' stock arrives from kashi's branch_stock feed; when a pull
// applied some, reload so the quantities shown are the new ones.
let branchStockSyncUnsub: (() => void) | null = null;
onMounted(() => {
  resetList();
  branchStockSyncUnsub = EventsOn("sync_applied:branch_stock", () => resetList());
});

onBeforeUnmount(() => {
  clearTimeout(searchTimer);
  latestRequest++;
  branchStockSyncUnsub?.();
});
</script>

<style scoped>
.inventory-page {
  display: flex;
  flex-direction: column;
  height: 100vh;
  min-height: 0;
}

.inventory-list {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.inventory-virtual-scroll {
  flex: 1 1 auto;
  min-height: 0;
}

.inventory-list-item {
  border-bottom: thin solid rgba(var(--k-border-color), var(--k-border-opacity));
}

.inventory-row {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.inventory-name {
  flex: 1 1 auto;
}

.inventory-code,
.inventory-meta,
.inventory-variant {
  font-size: 0.75rem;
}

.inventory-code,
.inventory-meta {
  flex: 0 0 96px;
  width: 96px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inventory-branches {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.inventory-price {
  flex: 0 0 72px;
  text-align: end;
  font-weight: 600;
}

.inventory-status {
  flex: 0 0 auto;
  min-height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.8125rem;
}

.inventory-status-error {
  flex-direction: column;
  gap: 2px;
  padding: 8px 16px;
}

.inventory-search-row {
  flex: 0 0 auto;
  padding: 8px 16px 6px;
}

/* Tall enough for the full "code" layout keyboard, as in the dialog. */
.inventory-keyboard {
  flex: 0 0 264px;
  height: 264px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  margin-bottom: 16px;
  margin-inline: auto;
  width: 100%;
  max-width: 748px;
  padding-inline: 8px;
  padding-top: 4px;
}

@media (max-width: 900px) {
  .inventory-meta,
  .inventory-variant {
    display: none;
  }
}
</style>
