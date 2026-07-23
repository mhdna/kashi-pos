<script setup lang="ts">
import { ref } from "vue";
import ServerSideTable from "@/components/ServerSideTable.vue";

const apiURL = "http://localhost:4123/products";
const headers = ref([
  { title: "ID", key: "id", align: "start" },
  { title: "Code", key: "code", align: "start" },
  { title: "Name", key: "name", align: "start" },
  { title: "Description", key: "description", align: "start" },
  { title: "Active", key: "is_active", align: "center" },
  { title: "Price", key: "price", align: "end" },
  { title: "Discount", key: "discount", align: "end" },
  { title: "Created At", key: "created_at", align: "end" },
]);

const menuOpen = ref(false);
</script>

<template>
  <div class="d-flex h-100 w-100">
    <v-card
      height="100%"
      style="flex: 1"
      title="This Shift's Total Balance"
      width="100%"
    >
      <v-card-text>
        <div>Total:</div>
        <div>Total Paypal:</div>
        <div>Total LBP:</div>
      </v-card-text>
    </v-card>
    <v-card height="100%" style="flex: 2" width="100%">
      <v-menu v-model="menuOpen">
        <template v-slot:activator="{ props }">
          <v-btn color="primary" v-bind="props" class="ma-2">Date</v-btn>
        </template>
        <v-date-picker elevation="5"></v-date-picker>
      </v-menu>

      <ServerSideTable
        :api-u-r-l="apiURL"
        :header="headers"
        root-key="products"
      />
    </v-card>
  </div>
</template>
