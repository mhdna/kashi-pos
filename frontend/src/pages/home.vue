<template>
  <div class="d-flex">
    <div style="flex: 0 0 50%;">
      <v-sheet>
        <div>
            <return-invoice-menu />
          </v-card-title>
          <v-card-subtitle>
            SA/CN/2026/1441
          </v-card-subtitle>
        </div>
        <v-text-field class="mx-4 mt-4" variant="solo-filled" hide-details="auto" label="Barcode"></v-text-field>
        <InvoicesTable />
      </v-sheet>
    </div>
    <div style="flex: 0 0 30%;">
      <Calculator />
    </div>
    <div style="flex: 0 0 20%;">
      <ActionBar />
    </div>
  </div>
</template>

<script lang="ts" setup>
import InvoicesTable from '@/components/InvoicesTable.vue';
import ActionBar from '@/components/ActionBar.vue';
import Calculator from '@/components/Calculator.vue';
import ReturnInvoiceMenu from '@/components/ReturnInvoiceMenu.vue';
</script>
