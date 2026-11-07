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
            <k-list-item
              class="on-hold-item text-left"
              :class="`on-hold-item--${holdKind(h).kind}`"
              :data-kind="holdKind(h).kind"
              @click="restoreHold(h)"
            >
              <template #prepend>
                <k-avatar :color="holdKind(h).color" variant="tonal" size="36" class="on-hold-avatar">
                  <k-icon :icon="holdKind(h).icon" size="18" />
                </k-avatar>
              </template>
              <k-list-item-title class="font-weight-medium d-flex align-center ga-2">
                <k-chip
                  class="on-hold-kind"
                  :color="holdKind(h).color"
                  label
                  size="x-small"
                  variant="flat"
                  :text="holdKind(h).label"
                />
                <span class="on-hold-title">{{ holdTitle(h) }}</span>
              </k-list-item-title>
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
import { holdKind, holdTitle } from "@/lib/holdSnapshot";

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
/* A stripe down the left in the hold's colour: blue for a sale, the
   exchange's orange for an exchange. */
.on-hold-item {
  border-inline-start: 4px solid transparent;
}
.on-hold-item--sale {
  border-inline-start-color: rgb(var(--k-theme-primary));
}
.on-hold-item--exchange {
  border-inline-start-color: #ef6c00; /* orange-darken-3 (ui/palette.ts) */
  background-color: rgba(239, 108, 0, 0.06);
}
.on-hold-item--exchange:hover {
  background-color: rgba(239, 108, 0, 0.12);
}
.on-hold-title {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.on-hold-empty {
  min-height: 100%;
}
</style>
