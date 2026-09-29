<template>
  <v-card class="pa-4 mb-4">
    <v-card-title>Inventory</v-card-title>
    <v-data-table :headers="headers" :items="items" :loading="loading" density="compact" hover />
  </v-card>

  <v-card class="pa-4">
    <v-card-title>Other Branches</v-card-title>
    <v-card-subtitle>
      Search what another branch has on hand and its selling price. Kept live from kashi.
    </v-card-subtitle>
    <v-card-text>
      <v-text-field
        v-model="search"
        clearable
        density="compact"
        hide-details
        label="Search by product name, code, or barcode"
        prepend-inner-icon="mdi-magnify"
        variant="outlined"
        @update:model-value="onSearch"
      />
      <v-data-table
        class="mt-4"
        density="compact"
        :headers="branchStockHeaders"
        hover
        :items="branchStockRows"
        :items-per-page="25"
        :loading="branchStockLoading"
      >
        <template #item.priceCents="{ item }">{{ formatMoney(item.priceCents) }}</template>
        <template #no-data>
          {{ search ? "No matching stock at any other branch." : "Type to search other branches' stock." }}
        </template>
      </v-data-table>
    </v-card-text>
  </v-card>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from "vue";
import { ListInventories, ListBranchStock } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

const headers = [
  { title: "ID", key: "id" },
  { title: "Name", key: "name" },
];
const items = ref<any[]>([]);
const loading = ref(true);

onMounted(async () => {
  try {
    items.value = await ListInventories();
  } catch {}
  loading.value = false;
});

const branchStockHeaders = [
  { title: "Branch", key: "branchCode" },
  { title: "Product", key: "productName" },
  { title: "Code", key: "productCode" },
  { title: "Barcode", key: "barcode" },
  { title: "Quantity", key: "quantity" },
  { title: "Price", key: "priceCents" },
];
const search = ref("");
const branchStockRows = ref<any[]>([]);
const branchStockLoading = ref(false);

function formatMoney(cents: number) {
  return `$${(cents / 100).toFixed(2)}`;
}

let searchTimer: ReturnType<typeof setTimeout> | undefined;
function onSearch(value: string) {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => runSearch(value ?? ""), 250);
}

async function runSearch(value: string) {
  branchStockLoading.value = true;
  try {
    branchStockRows.value = await ListBranchStock(value);
  } catch {
    branchStockRows.value = [];
  } finally {
    branchStockLoading.value = false;
  }
}

// Kept live: kashi's branch_stock feed can land mid-search (another
// branch made a sale, kashi approved a transfer, ...), so re-run
// whatever's currently typed whenever a sync pull actually applied
// something -- same "go re-fetch this" contract every other synced
// entity's page already listens for (see loans.vue/expenses.vue).
let branchStockSyncUnsub: (() => void) | null = null;

onMounted(() => {
  branchStockSyncUnsub = EventsOn("sync_applied:branch_stock", () => runSearch(search.value ?? ""));
});

onBeforeUnmount(() => {
  branchStockSyncUnsub?.();
});
</script>
