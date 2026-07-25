<template>
  <v-card class="pa-4">
    <v-card-title>Clients</v-card-title>
    <v-data-table :headers="headers" :items="items" :loading="loading" density="compact" hover>
      <template #item.avatar="{ item }">
        <InitialsAvatar
          :name="item.name ?? ''"
          :color="colors[items.indexOf(item) % colors.length] ?? 'grey'"
          :size="32"
        />
      </template>
      <template #item.totalLoyaltyPoints="{ item }">
        <span>{{ item.totalLoyaltyPoints }} pts</span>
      </template>
    </v-data-table>
  </v-card>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
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
const loading = ref(true);

onMounted(async () => {
  try {
    items.value = await ListClients();
  } catch {}
  loading.value = false;
});
</script>
