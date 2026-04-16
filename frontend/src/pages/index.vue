<template>
  <AppBar />
  <NavDrawer />
  <!-- html, body, #app { -->
  <!-- height: 100%; -->
  <!-- overflow: hidden; -->
  <!-- } -->
  <v-container fluid style="height: 91vh; overflow: hidden;">
    <v-row fluid>
      <v-col cols="5">
        <v-card>
          <div>
            <v-card-title>
              Invoice
            </v-card-title>
            <v-card-subtitle>
              SA/CN/2026/1441
            </v-card-subtitle>
          </div>
          <v-text-field class="mx-4 mt-4" variant="solo-filled" hide-details="auto" label="Barcode"></v-text-field>
          <InvoicesTable />
        </v-card>
      </v-col>
      <v-col cols="4">
        <Calculator />
      </v-col>
      <v-col cols="3" class="w-100">
        <BottomBar />
      </v-col>
    </v-row>
  </v-container>
</template>

<script lang="ts" setup>
import AppBar from '@/components/AppBar.vue';
import NavDrawer from '@/components/NavDrawer.vue';
import InvoicesTable from '@/components/InvoicesTable.vue';
import BottomBar from '@/components/BottomBar.vue';
import Calculator from '@/components/Calculator.vue';
</script>
