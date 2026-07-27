<template>
  <v-card class="pa-4">
    <v-card-title class="d-flex align-center justify-space-between">
      Clients
      <v-text-field
        v-model="search"
        class="ms-4"
        clearable
        density="compact"
        hide-details
        label="Search"
        max-width="320"
        prepend-inner-icon="mdi-magnify"
        single-line
        variant="outlined"
      />
    </v-card-title>
    <v-data-table-server
      v-model:items-per-page="itemsPerPage"
      :headers="headers"
      :items="visibleItems"
      :items-length="totalItems"
      :loading="loading"
      density="compact"
      hover
      @update:options="loadItems"
    >
      <template #item.avatar="{ item }">
        <InitialsAvatar
          :name="item.name ?? ''"
          :color="colors[items.indexOf(item) % colors.length] ?? 'grey'"
        />
      </template>
      <template #item.totalLoyaltyPoints="{ item }">
        <span>{{ item.totalLoyaltyPoints }} pts</span>
      </template>
    </v-data-table-server>
  </v-card>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from "vue";
import { ListClients } from "../../wailsjs/go/main/App";
import InitialsAvatar from "@/components/InitialsAvatar.vue";

const colors = ["yellow", "blue", "red", "purple", "pink", "green", "orange", "teal"];

const headers = [
  { title: "", key: "avatar", sortable: false, width: 56 },
  { title: "Name", key: "name" },
  { title: "Phone", key: "phone" },
  { title: "Loyalty Points", key: "totalLoyaltyPoints" },
  { title: "Valid Points", key: "validLoyaltyPoints" },
  { title: "Created", key: "createdAt" },
];
const items = ref<any[]>([]);
const visibleItems = ref<any[]>([]);
const search = ref("");
const itemsPerPage = ref(10);
const totalItems = ref(0);
const loading = ref(true);

function loadItems(options: {
  page: number;
  itemsPerPage: number;
  sortBy: { key: string; order: string }[];
}) {
  let filtered = items.value;
  const query = search.value.trim().toLowerCase();
  if (query) {
    filtered = filtered.filter((item) =>
      [item.name, item.phone].some((value) =>
        String(value ?? "")
          .toLowerCase()
          .includes(query),
      ),
    );
  }
  const sort = options.sortBy[0];
  if (sort) {
    filtered = [...filtered].sort((a, b) => {
      const left = a[sort.key] ?? "";
      const right = b[sort.key] ?? "";
      return (left > right ? 1 : left < right ? -1 : 0) * (sort.order === "desc" ? -1 : 1);
    });
  }
  totalItems.value = filtered.length;
  const start = (options.page - 1) * options.itemsPerPage;
  visibleItems.value = filtered.slice(start, start + options.itemsPerPage);
}

watch([items, search, itemsPerPage], () =>
  loadItems({ page: 1, itemsPerPage: itemsPerPage.value, sortBy: [] }),
);

onMounted(async () => {
  try {
    items.value = await ListClients();
  } catch {
    items.value = [];
  }
  loading.value = false;
});
</script>
