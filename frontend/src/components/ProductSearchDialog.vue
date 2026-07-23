<template>
  <v-dialog v-model="dialog" max-width="700">
    <template #activator="{ props: activatorProps }">
      <v-btn
        v-bind="activatorProps"
        variant="text"
        size="small"
        density="compact"
        class="ms-1"
      >
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
      <v-data-table
        :headers="headers"
        :items="products"
        :search="search"
        :items-per-page="10"
        hover
        return-object
        @click:row="selectProduct"
      />
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
import { ref, inject, watch, onMounted } from "vue";
import { ListProducts } from "../../wailsjs/go/main/App";

const emit = defineEmits<{
  select: [product: { name: string; code: string; category: string; price: number; discount: number; qty: number }];
}>();

const dialogOpen = inject<import("vue").Ref<boolean>>("dialogOpen", { value: false } as any);
const dialog = ref(false);
const search = ref("");

watch(dialog, (v) => { dialogOpen.value = v; });

const headers = [
  { title: "Code", key: "code", align: "start" as const },
  { title: "Name", key: "name", align: "start" as const },
  { title: "Category", key: "category", align: "start" as const },
  { title: "Price", key: "price", align: "end" as const },
];

const products = ref<any[]>([]);

onMounted(async () => {
  try {
    products.value = (await ListProducts()).map((p) => ({ ...p, qty: 1 }));
  } catch {}
});

function selectProduct(_event: any, row: any) {
  emit("select", row.item);
  dialog.value = false;
  search.value = "";
}
</script>
