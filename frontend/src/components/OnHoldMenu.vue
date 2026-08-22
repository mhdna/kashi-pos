<template>
  <v-btn v-if="!hideActivator" class="px-5" variant="tonal" @click="menu = true">
    <template #prepend>
      <v-icon icon="mdi-pause" size="20" class="me-2" />
    </template>
    On Hold
  </v-btn>

  <v-dialog v-model="menu" max-width="500">
    <v-card class="on-hold-card d-flex flex-column text-center" height="480" rounded="lg">
      <v-card-title class="d-flex flex-column align-center pa-4 flex-shrink-0">
        <v-avatar color="primary" variant="tonal" size="56" class="mb-2">
          <v-icon icon="mdi-pause" size="28" />
        </v-avatar>
        <div class="text-subtitle-1 font-weight-bold">Invoices on hold</div>
        <div class="text-caption text-medium-emphasis">{{ holds.length }} held</div>
      </v-card-title>
      <v-divider />
      <v-card-text class="on-hold-list pa-0 flex-grow-1 overflow-y-auto">
        <v-list v-if="holds.length" class="py-0">
          <template v-for="(h, index) in holds" :key="h.id">
            <v-list-item class="on-hold-item text-left" @click="restoreHold(h)">
              <template #prepend>
                <v-avatar color="primary" variant="tonal" size="36">
                  <v-icon icon="mdi-receipt-text-outline" size="18" />
                </v-avatar>
              </template>
              <v-list-item-title class="font-weight-medium">#{{ h.id }} — {{ h.itemCount }} items</v-list-item-title>
              <v-list-item-subtitle>{{ formatServerDateTime(h.createdAt) }}</v-list-item-subtitle>
              <template #append>
                <v-btn icon size="small" variant="text" aria-label="Remove hold" @click.stop="removeHold(h.id)">
                  <v-icon icon="mdi-close" size="16" />
                </v-btn>
              </template>
            </v-list-item>
            <v-divider v-if="index < holds.length - 1" />
          </template>
        </v-list>
        <div v-else class="on-hold-empty d-flex flex-column align-center justify-center h-100 text-medium-emphasis">
          <v-icon icon="mdi-tray-remove" size="40" class="mb-2" />
          <div class="text-body-2">No invoices on hold</div>
        </div>
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { ListHoldInvoices, DeleteHoldInvoice } from "../../wailsjs/go/main/App";
import { formatServerDateTime } from "@/lib/datetime";

defineProps<{ hideActivator?: boolean }>();
const emit = defineEmits<{ restore: [hold: any] }>();

const menu = ref(false);
const holds = ref<any[]>([]);

defineExpose({ open: () => { menu.value = true; }, close: () => { menu.value = false; } });

async function load() {
  holds.value = await ListHoldInvoices();
}
function restoreHold(hold: any) {
  emit("restore", hold);
  holds.value = holds.value.filter((h) => h.id !== hold.id);
  menu.value = false;
}
async function removeHold(id: number) {
  await DeleteHoldInvoice(id);
  await load();
}

watch(menu, async (v) => {
  if (v) await load();
});
</script>

<style scoped>
.on-hold-item {
  cursor: pointer;
  transition: background-color 120ms ease;
}
.on-hold-item:hover {
  background-color: rgba(var(--v-theme-on-surface), 0.04);
}
.on-hold-empty {
  min-height: 100%;
}
</style>
