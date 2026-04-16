<template>
  <v-data-table :custom-filter="filterOnlyCapsText" :items-per-page="-1" hide-default-footer :headers="headers"
    :items="items" :search="search" item-value="name">
    <template v-slot:top>
      <!-- <v-text-field -->
      <!--   v-model="search" -->
      <!--   class="pa-2" -->
      <!--   label="Search (UPPER CASE ONLY)" -->
      <!-- ></v-text-field> -->
    </template>
  </v-data-table>
</template>
<script setup>
import { ref } from 'vue'

const search = ref('')
const headers = [
  {
    title: 'Description',
    align: 'start',
    key: 'name',
  },
  {
    title: 'Code',
    align: 'end',
    key: 'cores',
  },
  {
    title: 'Qty',
    align: 'end',
    key: 'threads',
  },
  {
    title: 'Color',
    align: 'end',
    key: 'baseClock',
  },
  {
    title: 'Discount',
    align: 'end',
    key: 'boostClock',
  },
  {
    title: 'Net Amount',
    align: 'end',
    key: 'tdp',
  },
]
const items = [
  {
    name: 'Intel Core i9-11900K',
    cores: 8,
    threads: 16,
    baseClock: '3.5 GHz',
    boostClock: '5.3 GHz',
    tdp: '125W',
  },
  {
    name: 'AMD Ryzen 9 5950X',
    cores: 16,
    threads: 32,
    baseClock: '3.4 GHz',
    boostClock: '4.9 GHz',
    tdp: '105W',
  },
  {
    name: 'Intel Core i7-10700K',
    cores: 8,
    threads: 16,
    baseClock: '3.8 GHz',
    boostClock: '5.1 GHz',
    tdp: '125W',
  },
  {
    name: 'AMD Ryzen 5 5600X',
    cores: 6,
    threads: 12,
    baseClock: '3.7 GHz',
    boostClock: '4.6 GHz',
    tdp: '65W',
  },
  {
    name: 'Intel Core i5-10600K',
    cores: 6,
    threads: 12,
    baseClock: '4.1 GHz',
    boostClock: '4.8 GHz',
    tdp: '125W',
  },
  {
    name: 'AMD Ryzen 7 5800X',
    cores: 8,
    threads: 16,
    baseClock: '3.8 GHz',
    boostClock: '4.7 GHz',
    tdp: '105W',
  },
  {
    name: 'Intel Core i3-10100',
    cores: 4,
    threads: 8,
    baseClock: '3.6 GHz',
    boostClock: '4.3 GHz',
    tdp: '65W',
  },
  {
    name: 'AMD Ryzen 3 3300X',
    cores: 4,
    threads: 8,
    baseClock: '3.8 GHz',
    boostClock: '4.3 GHz',
    tdp: '65W',
  },
  {
    name: 'Intel Pentium Gold G6400',
    cores: 2,
    threads: 4,
    baseClock: '4.0 GHz',
    tdp: '58W',
  },
  {
    name: 'AMD Athlon 3000G',
    cores: 2,
    threads: 4,
    baseClock: '3.5 GHz',
    tdp: '35W',
  },
  {
    name: 'AMD Ryzen 3 3300X',
    cores: 4,
    threads: 8,
    baseClock: '3.8 GHz',
    boostClock: '4.3 GHz',
    tdp: '65W',
  },
  {
    name: 'AMD Ryzen 3 3300X',
    cores: 4,
    threads: 8,
    baseClock: '3.8 GHz',
    boostClock: '4.3 GHz',
    tdp: '65W',
  },
  {
    name: 'AMD Ryzen 3 3300X',
    cores: 4,
    threads: 8,
    baseClock: '3.8 GHz',
    boostClock: '4.3 GHz',
    tdp: '65W',
  },
  {
    name: 'AMD Ryzen 3 3300X',
    cores: 4,
    threads: 8,
    baseClock: '3.8 GHz',
    boostClock: '4.3 GHz',
    tdp: '65W',
  },
  {
    name: 'AMD Ryzen 3 3300X',
    cores: 4,
    threads: 8,
    baseClock: '3.8 GHz',
    boostClock: '4.3 GHz',
    tdp: '65W',
  },
  {
    name: 'AMD Ryzen 3 3300X',
    cores: 4,
    threads: 8,
    baseClock: '3.8 GHz',
    boostClock: '4.3 GHz',
    tdp: '65W',
  },
  {
    name: 'AMD Ryzen 3 3300X',
    cores: 4,
    threads: 8,
    baseClock: '3.8 GHz',
    boostClock: '4.3 GHz',
    tdp: '65W',
  },
  {
    name: 'AMD Ryzen 3 3300X',
    cores: 4,
    threads: 8,
    baseClock: '3.8 GHz',
    boostClock: '4.3 GHz',
    tdp: '65W',
  },
  {
    name: 'AMD Ryzen 3 3300X',
    cores: 4,
    threads: 8,
    baseClock: '3.8 GHz',
    boostClock: '4.3 GHz',
    tdp: '65W',
  },
  {
    name: 'AMD Ryzen 3 3300X',
    cores: 4,
    threads: 8,
    baseClock: '3.8 GHz',
    boostClock: '4.3 GHz',
    tdp: '65W',
  },
  {
    name: 'AMD Ryzen 3 3300X',
    cores: 4,
    threads: 8,
    baseClock: '3.8 GHz',
    boostClock: '4.3 GHz',
    tdp: '65W',
  },
  {
    name: 'AMD Ryzen 3 3300X',
    cores: 4,
    threads: 8,
    baseClock: '3.8 GHz',
    boostClock: '4.3 GHz',
    tdp: '65W',
  },
  {
    name: 'AMD Athlon 3000G',
    cores: 2,
    threads: 4,
    baseClock: '3.5 GHz',
    tdp: '35W',
  },
  {
    name: 'AMD Athlon 3000G',
    cores: 2,
    threads: 4,
    baseClock: '3.5 GHz',
    tdp: '35W',
  },
]

function filterOnlyCapsText(value, query, item) {
  return value != null && query != null && typeof value === 'string' && value.toString().toLocaleUpperCase().indexOf(query) !== -1
}
</script>