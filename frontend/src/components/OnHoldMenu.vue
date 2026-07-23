<template>
  <v-menu v-model="menu" :close-on-content-click="false" location="bottom">
    <template #activator="{ props }">
      <v-btn v-bind="props" class="px-5" variant="tonal">
        <template #prepend>
          <v-icon icon="mdi-pause" size="20" class="me-2" />
        </template>
        On Hold
      </v-btn>
    </template>
    <v-card
      class="d-flex flex-column text-center"
      min-height="200"
      min-width="300"
      subtitle="Invoices On Hold"
    >
      <v-card-text class="pa-0 flex-grow-1 overflow-y-auto">
        <v-list max-height="300">
          <v-list-item> hi </v-list-item>
          <v-list-item> hi </v-list-item>
          <v-list-item> hi </v-list-item>
          <v-list-item> hi </v-list-item>
          <v-list-item> hi </v-list-item>
          <v-list-item> hi </v-list-item>
          <v-list-item> hi </v-list-item>
          <v-list-item> hi </v-list-item>
        </v-list>
      </v-card-text>

      <v-card-actions class="pa-2">
        <v-btn variant="tonal" block>Set Up</v-btn>
      </v-card-actions>
    </v-card>
  </v-menu>
</template>

<script setup lang="ts">
import { ref } from "vue";
const menu = ref(false);
</script>
