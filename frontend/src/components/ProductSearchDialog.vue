<template>
  <v-dialog v-model="dialog" max-width="760" width="calc(100% - 32px)" height="670">
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
        class="product-list-table"
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

function openDialog() {
  dialog.value = true;
}

defineExpose({ openDialog });
</script>

<style scoped>
.product-search-control {
  max-width: 320px;
  width: 100%;
}

.product-list-card {
  height: 670px;
  display: flex;
  flex-direction: column;
}

.product-list-table {
  flex: 0 0 400px;
  height: 400px;
  min-height: 0;
  overflow-y: auto;
}

.product-inline-keyboard {
  flex: 0 0 205px;
  height: 205px;
  display: flex;
  align-items: center;
  margin-inline: auto;
  max-width: 600px;
  padding-block: 12px;
}
</style>
