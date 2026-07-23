<template>
  <v-menu v-model="menu" :close-on-content-click="false" location="bottom">
    <template #activator="{ props }">
      <v-btn v-bind="props" class="px-5" variant="tonal">
        <template #prepend>
          <v-icon icon="mdi-autorenew" size="20" class="me-2" />
        </template>
        Return Invoice
      </v-btn>
    </template>
    <v-card
      class="d-flex flex-column text-center"
      min-width="400"
      subtitle="Return Invoice"
    >
      <v-card-text class="flex-grow-1">
        <v-text-field
          density="compact"
          label="Sales Invoice Barcode"
          variant="outlined"
          prepend-inner-icon="mdi-barcode"
        >
        </v-text-field>
      </v-card-text>
      <v-card-actions class="pa-2 ga-2">
        <v-btn class="flex-1-1" variant="elevated">Cancel</v-btn>
        <v-btn class="flex-1-1" color="red-darken-4" variant="elevated"
          >Return Invoice</v-btn
        >
      </v-card-actions>
    </v-card>
  </v-menu>
</template>

<script setup lang="ts">
import { ref } from "vue";
const menu = ref(false);
</script>
