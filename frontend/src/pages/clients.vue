<template>
  <v-card class="pa-4 ma-4">
    <v-card-title>Clients</v-card-title>
    <v-data-table :headers="headers" :items="items" :loading="loading" density="compact" hover>
      <template #item.totalLoyaltyPoints="{ item }">
        <span>{{ item.totalLoyaltyPoints }} pts</span>
      </template>
    </v-data-table>
  </v-card>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { ListClients } from "../../wailsjs/go/main/App";

const headers = [
  { title: "Name", key: "name" },
  { title: "Phone", key: "phone" },
  { title: "Loyalty Points", key: "totalLoyaltyPoints" },
  { title: "Valid Points", key: "validLoyaltyPoints" },
  { title: "Created", key: "createdAt" },
];
const items = ref<any[]>([]);
const loading = ref(true);

onMounted(async () => {
  try { items.value = await ListClients(); } catch {}
  loading.value = false;
});
</script>
