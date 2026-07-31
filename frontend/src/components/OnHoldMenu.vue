<template>
  <v-btn v-if="!hideActivator" class="px-5" variant="tonal" @click="menu = true">
    <template #prepend>
      <v-icon icon="mdi-pause" size="20" class="me-2" />
    </template>
    On Hold
  </v-btn>

  <v-dialog v-model="menu" max-width="500">
    <v-card class="d-flex flex-column text-center" min-height="200">
      <v-card-title>Invoices On Hold</v-card-title>
      <v-card-text class="pa-0 flex-grow-1 overflow-y-auto">
        <v-list v-if="holds.length" max-height="300">
          <v-list-item v-for="h in holds" :key="h.id" @click="restoreHold(h.id, h.items)">
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
  </v-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { ListHoldInvoices, DeleteHoldInvoice } from "../../wailsjs/go/main/App";

defineProps<{ hideActivator?: boolean }>();
const emit = defineEmits<{ restore: [id: number, items: any[]] }>();

const menu = ref(false);
const holds = ref<any[]>([]);

defineExpose({ open: () => { menu.value = true; }, close: () => { menu.value = false; } });

async function load() {
  holds.value = await ListHoldInvoices();
}
function restoreHold(id: number, items: any[]) {
  emit("restore", id, items);
  holds.value = holds.value.filter((h) => h.id !== id);
  if (!holds.value.length) menu.value = false;
}
async function removeHold(id: number) {
  await DeleteHoldInvoice(id);
  await load();
}

watch(menu, async (v) => {
  if (v) await load();
});
</script>
