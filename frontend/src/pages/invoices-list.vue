<template>
  <v-card class="pa-4 ma-4">
    <v-card-title>Invoices</v-card-title>
    <v-data-table :headers="headers" :items="items" :loading="loading" density="compact" hover>
      <template #item.grandTotal="{ item }">${{ (item.grandTotal / 100).toFixed(2) }}</template>
      <template #item.createdAt="{ item }">{{ item.createdAt?.slice(0, 19) }}</template>
    </v-data-table>
  </v-card>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { ListInvoices } from "../../wailsjs/go/main/App";

const headers = [
  { title: "Code", key: "invoiceCode" },
  { title: "Client", key: "clientId" },
  { title: "Subtotal", key: "subtotal" },
  { title: "Discount", key: "discountedTotal" },
  { title: "Total", key: "grandTotal" },
  { title: "Date", key: "createdAt" },
];
const items = ref<any[]>([]);
const loading = ref(true);

onMounted(async () => {
  try { items.value = await ListInvoices(); } catch {}
  loading.value = false;
});
</script>
