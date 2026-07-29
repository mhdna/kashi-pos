<template>
  <v-dialog v-model="dialog" max-width="760" min-height="440">
    <template #activator="{ props: activatorProps }">
      <v-btn v-bind="activatorProps" variant="text" class="ms-1">
        <v-icon icon="mdi-format-list-bulleted-square" size="20" class="me-1" />
        {{ activatorText }}
      </v-btn>
    </template>
    <v-card>
      <v-card-title class="d-flex align-center">
        Products
        <v-spacer />
        <v-text-field
          v-model="search"
          class="product-search-control ms-4"
          density="compact"
          hide-details
          placeholder="Search products..."
          variant="outlined"
          autofocus
        />
      </v-card-title>
      <v-data-table-server
        v-model:items-per-page="itemsPerPage"
        :headers="headers"
        :items="serverItems"
        :items-length="totalItems"
        :loading="loading"
        :search="debouncedSearch"
        hover
        return-object
        @click:row="selectProduct"
        @update:options="loadItems"
      />
      <v-divider />
      <v-card-text class="product-inline-keyboard">
        <InlineKeyboard v-model="search" :max-length="80" layout="code" />
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
import { ref, inject, onBeforeUnmount, watch } from "vue";
import { ListProductsPage } from "../../wailsjs/go/main/App";
import InlineKeyboard from "@/components/InlineKeyboard.vue";

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

const props = withDefaults(defineProps<{ activatorText?: string }>(), {
  activatorText: "Products List",
});
const activatorText = props.activatorText;

const dialogOpen = inject<import("vue").Ref<boolean>>("dialogOpen", { value: false } as any);
const dialog = ref(false);
const search = ref("");
const debouncedSearch = ref("");
let searchTimer: ReturnType<typeof setTimeout> | undefined;
let latestRequest = 0;

watch(dialog, (v) => {
  dialogOpen.value = v;
});

watch(search, (value) => {
  clearTimeout(searchTimer);
  latestRequest++;
  searchTimer = setTimeout(() => {
    debouncedSearch.value = value.trim();
  }, 250);
});

const headers = [
  { title: "Code", key: "code", align: "start" as const },
  { title: "Name", key: "name", align: "start" as const },
  // { title: "Category", key: "category", align: "start" as const },
  { title: "Price", key: "price", align: "end" as const },
];

const itemsPerPage = ref(5);
const serverItems = ref<any[]>([]);
const totalItems = ref(0);
const loading = ref(false);

async function loadItems({
  page,
  itemsPerPage: requestedItemsPerPage,
}: {
  page: number;
  itemsPerPage: number;
}) {
  const request = ++latestRequest;
  loading.value = true;
  try {
    const result = await ListProductsPage(page, requestedItemsPerPage, debouncedSearch.value);
    if (request !== latestRequest) return;
    serverItems.value = result.items.map((p) => ({ ...p, qty: 1 }));
    totalItems.value = result.total;
  } catch (e) {
    if (request !== latestRequest) return;
    console.error("ListProductsPage failed:", e);
    serverItems.value = [];
    totalItems.value = 0;
  } finally {
    if (request === latestRequest) loading.value = false;
  }
}

function selectProduct(_event: any, row: any) {
  emit("select", row.item);
  dialog.value = false;
  search.value = "";
}

onBeforeUnmount(() => {
  clearTimeout(searchTimer);
  latestRequest++;
});
</script>

<style scoped>
.product-search-control {
  max-width: 320px;
  width: 100%;
}

.product-inline-keyboard {
  margin-inline: auto;
  max-width: 600px;
  padding-block: 12px;
}
</style>
