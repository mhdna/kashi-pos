<template>
  <v-card class="pa-4 ma-4">
    <v-card-title>Inventory</v-card-title>
    <v-data-table :headers="headers" :items="items" :loading="loading" density="compact" hover />
  </v-card>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { ListInventories } from "../../wailsjs/go/main/App";

const headers = [
  { title: "ID", key: "id" },
  { title: "Name", key: "name" },
];
const items = ref<any[]>([]);
const loading = ref(true);

onMounted(async () => {
  try { items.value = await ListInventories(); } catch {}
  loading.value = false;
});
</script>
