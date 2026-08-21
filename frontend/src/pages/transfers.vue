<template>
  <v-card class="pa-4">
    <v-card-title>Transfers</v-card-title>
    <v-data-table :headers="headers" :items="items" :loading="loading" density="compact" hover>
      <template #item.createdAt="{ item }">{{ formatServerDateTime(item.createdAt) }}</template>
    </v-data-table>
  </v-card>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { ListTransfers } from "../../wailsjs/go/main/App";
import { formatServerDateTime } from "@/lib/datetime";

const headers = [
  { title: "ID", key: "id" },
  { title: "From", key: "fromId" },
  { title: "To", key: "toId" },
  { title: "Type", key: "type" },
  { title: "Date", key: "createdAt" },
];
const items = ref<any[]>([]);
const loading = ref(true);

onMounted(async () => {
  try {
    items.value = await ListTransfers();
  } catch {}
  loading.value = false;
});
</script>
