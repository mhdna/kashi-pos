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
    <v-card class="d-flex flex-column text-center" min-height="200" min-width="300" subtitle="Invoices On Hold">
      <v-card-text class="pa-0 flex-grow-1 overflow-y-auto">
        <v-list v-if="holds.length" max-height="300">
          <v-list-item v-for="h in holds" :key="h.id" @click="$emit('restore', h.items)">
            <v-list-item-title>#{{ h.id }} — {{ h.itemCount }} items</v-list-item-title>
            <v-list-item-subtitle>{{ h.createdAt?.slice(0, 19) }}</v-list-item-subtitle>
            <template #append>
              <v-btn icon size="small" variant="text" @click.stop="removeHold(h.id)">
                <v-icon icon="mdi-close" size="16" />
              </v-btn>
            </template>
          </v-list-item>
        </v-list>
        <v-list-item v-else>No invoices on hold</v-list-item>
      </v-card-text>
    </v-card>
  </v-menu>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { ListHoldInvoices, DeleteHoldInvoice } from "../../wailsjs/go/main/App";

defineEmits<{ restore: [items: any[]] }>();

const menu = ref(false);
const holds = ref<any[]>([]);

async function load() { holds.value = await ListHoldInvoices(); }
async function removeHold(id: number) { await DeleteHoldInvoice(id); await load(); }

watch(menu, async (v) => { if (v) await load(); });
</script>
