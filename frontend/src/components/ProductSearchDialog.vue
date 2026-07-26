<template>
  <v-dialog v-model="dialog" max-width="700" min-height="440">
    <template #activator="{ props: activatorProps }">
      <v-btn v-bind="activatorProps" variant="text" class="ms-1">
        <v-icon icon="mdi-format-list-bulleted-square" size="20" class="me-1" />
        Products List
      </v-btn>
    </template>
    <v-card>
      <v-card-title class="d-flex align-center">
        Products
        <v-spacer />
        <v-text-field
          v-model="search"
          density="compact"
          hide-details
          placeholder="Search products..."
          variant="outlined"
          class="ms-4"
          style="max-width: 250px"
          autofocus
        />
      </v-card-title>
      <v-data-table-server
        v-model:items-per-page="itemsPerPage"
        :headers="headers"
        :items="serverItems"
        :items-length="totalItems"
        :loading="loading"
        :search="search"
        hover
        return-object
        @click:row="selectProduct"
        @update:options="loadItems"
      />
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
import { ref, inject, watch } from "vue";
import { ListProductsPage } from "../../wailsjs/go/main/App";

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

const dialogOpen = inject<import("vue").Ref<boolean>>("dialogOpen", { value: false } as any);
const dialog = ref(false);
const search = ref("");

watch(dialog, (v) => {
  dialogOpen.value = v;
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

async function loadItems({ page, itemsPerPage: requestedItemsPerPage }: { page: number; itemsPerPage: number }) {
  loading.value = true;
  try {
    const result = await ListProductsPage(page, requestedItemsPerPage, search.value);
    serverItems.value = result.items.map((p) => ({ ...p, qty: 1 }));
    totalItems.value = result.total;
  } catch (e) {
    console.error("ListProductsPage failed:", e);
    serverItems.value = [];
    totalItems.value = 0;
  } finally {
    loading.value = false;
  }
}

function selectProduct(_event: any, row: any) {
  emit("select", row.item);
  dialog.value = false;
  search.value = "";
}
</script>
