<template>
  <k-btn v-if="!hideActivator" class="px-5" variant="tonal" @click="menu = true">
    <template #prepend>
      <k-icon icon="mdi-pause" size="20" class="me-2" />
    </template>
    On Hold
  </k-btn>

  <k-dialog v-model="menu" max-width="500">
    <k-card class="on-hold-card d-flex flex-column text-center" height="480" rounded="lg">
      <k-card-title class="d-flex flex-column align-center pa-4 flex-shrink-0">
        <k-avatar color="primary" variant="tonal" size="56" class="mb-2">
          <k-icon icon="mdi-pause" size="28" />
        </k-avatar>
        <div class="text-subtitle-1 font-weight-bold">Invoices on hold</div>
        <div class="text-caption text-medium-emphasis">{{ holds.length }} held</div>
      </k-card-title>
      <k-divider />
      <k-card-text class="on-hold-list pa-0 flex-grow-1 overflow-y-auto">
        <k-list v-if="holds.length" class="py-0">
          <template v-for="(h, index) in holds" :key="h.id">
            <k-list-item class="on-hold-item text-left" @click="restoreHold(h)">
              <template #prepend>
                <k-avatar color="primary" variant="tonal" size="36">
                  <k-icon icon="mdi-receipt-text-outline" size="18" />
                </k-avatar>
              </template>
              <k-list-item-title class="font-weight-medium">#{{ h.id }} — {{ h.itemCount }} items</k-list-item-title>
              <k-list-item-subtitle>{{ formatServerDateTime(h.createdAt) }}</k-list-item-subtitle>
              <template #append>
                <k-btn icon size="small" variant="text" aria-label="Remove hold" @click.stop="removeHold(h.id)">
                  <k-icon icon="mdi-close" size="16" />
                </k-btn>
              </template>
            </k-list-item>
            <k-divider v-if="index < holds.length - 1" />
          </template>
        </k-list>
        <div v-else class="on-hold-empty d-flex flex-column align-center justify-center h-100 text-medium-emphasis">
          <k-icon icon="mdi-tray-remove" size="40" class="mb-2" />
          <div class="text-body-2">No invoices on hold</div>
        </div>
      </k-card-text>
    </k-card>
  </k-dialog>
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
  background-color: rgba(var(--k-theme-on-surface), 0.04);
}
.on-hold-empty {
  min-height: 100%;
}
</style>
