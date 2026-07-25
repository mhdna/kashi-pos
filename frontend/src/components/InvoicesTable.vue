<template>
  <v-data-table
    :custom-filter="filterOnlyCapsText"
    :headers="headers"
    hide-default-footer
    item-value="name"
    :items="items"
    :items-per-page="-1"
    :search="search"
  />
</template>
<script setup>
import { ref } from "vue";

const search = ref("");
const headers = [
  {
    title: "Description",
    align: "start",
    key: "name",
  },
  {
    title: "Code",
    align: "end",
    key: "cores",
  },
  {
    title: "Qty",
    align: "end",
    key: "threads",
  },
  {
    title: "Discount",
    align: "end",
    key: "boostClock",
  },
  {
    title: "Net Amount",
    align: "end",
    key: "tdp",
  },
];
const items = [];

function filterOnlyCapsText(value, query, item) {
  return (
    value != null &&
    query != null &&
    typeof value === "string" &&
    value.toString().toLocaleUpperCase().includes(query)
  );
}
</script>
