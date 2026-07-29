<template>
  <section class="quick-access" :class="{ 'quick-access--open': open }">
    <div class="quick-access__toolbar">
      <v-card
        class="quick-access__toggle ps-2"
        elevation="0"
        rounded="lg"
        :ripple="false"
        @click="open = !open"
      >
        <v-icon :icon="open ? 'mdi-arrow-down' : 'mdi-button-pointer'" size="18" />
        <span v-show="!open" class="text-caption ms-1">Quick access</span>
      </v-card>
      <span v-if="open" class="text-caption font-weight-bold flex-grow-1">Quick access</span>
      <ProductSearchDialog v-if="open" activator-text="Add products" @select="addPinnedItem" />
    </div>
    <div v-if="open" class="quick-access__body">
      <div v-if="!items.length" class="text-caption text-medium-emphasis text-center pa-2">
        Add frequently used items here.
      </div>
      <div class="quick-access__items">
        <v-card
          v-for="item in items"
          :key="item.code"
          class="quick-access__item pa-2"
          elevation="1"
          rounded="lg"
          :aria-label="item.name"
          @click="addToInvoice(item)"
        >
          <div class="quick-access__name text-caption text-truncate">{{ item.name }}</div>
          <div class="quick-access__code text-caption text-truncate">{{ item.code }}</div>
          <div class="text-caption text-medium-emphasis">${{ Number(item.price).toFixed(2) }}</div>
          <v-btn
            icon
            size="x-small"
            variant="text"
            class="quick-access__remove"
            title="Remove item"
            @click.stop.prevent="removePinnedItem(item.code)"
          >
            <v-icon icon="mdi-close" size="14" />
          </v-btn>
        </v-card>
      </div>
    </div>
  </section>

  <v-dialog v-model="removeDialog" max-width="360">
    <v-card>
      <v-card-title>Remove quick-access item?</v-card-title>
      <v-card-text
        >Remove <strong>{{ removalName }}</strong> from quick access?</v-card-text
      >
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="removeDialog = false">Cancel</v-btn>
        <v-btn color="error" variant="tonal" @click="confirmRemoval">Remove</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
import { ref, watch } from "vue";
import ProductSearchDialog from "@/components/ProductSearchDialog.vue";
import { GetProductByCode } from "../../wailsjs/go/main/App";

type QuickAccessItem = {
  name: string;
  code: string;
  category: string;
  price: number;
  discount: number;
  qty: number;
  barcode?: string;
};

const emit = defineEmits<{ add: [item: QuickAccessItem] }>();
const STORAGE_KEY = "kashi-pos.quick-access";
const items = ref<QuickAccessItem[]>([]);
const open = ref(true);
const removeDialog = ref(false);
const removalCode = ref("");
const removalName = ref("");

try {
  const saved = JSON.parse(localStorage.getItem(STORAGE_KEY) || "{}") as {
    items?: QuickAccessItem[];
    open?: boolean;
  };
  items.value = saved.items ?? [];
  open.value = saved.open ?? true;
} catch {}

watch(
  [items, open],
  () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ items: items.value, open: open.value }));
  },
  { deep: true },
);

function addPinnedItem(item: QuickAccessItem) {
  if (!items.value.some((existing) => existing.code === item.code))
    items.value.push({ ...item, qty: 1 });
}
async function addToInvoice(item: QuickAccessItem) {
  try {
    const latest = (await GetProductByCode(item.code)) as QuickAccessItem;
    Object.assign(item, latest);
  } catch {}
  emit("add", { ...item, qty: 1 });
}
function removePinnedItem(code: string) {
  const item = items.value.find((entry) => entry.code === code);
  if (!item) return;
  removalCode.value = code;
  removalName.value = item.name;
  removeDialog.value = true;
}
function confirmRemoval() {
  items.value = items.value.filter((entry) => entry.code !== removalCode.value);
  removeDialog.value = false;
  removalCode.value = "";
  removalName.value = "";
}
</script>

<style scoped>
.quick-access {
  border: 1px solid rgba(var(--v-border-color), 0.12);
  border-radius: 12px;
  display: flex;
  flex: 0 0 42px;
  flex-direction: column;
  margin: 0 8px 8px 16px;
  overflow: hidden;
  transition: none !important;
}
.quick-access--open {
  flex-basis: 168px;
}
.quick-access:not(.quick-access--open) {
  align-self: flex-start;
  border: 0;
  margin-left: 16px;
  width: auto;
}
.quick-access__toolbar {
  align-items: center;
  display: flex;
  min-height: 40px;
}
.quick-access__toggle {
  align-items: center;
  cursor: pointer;
  display: flex;
  min-height: 36px;
  overflow: hidden;
  transition: none !important;
  width: 40px;
}
.quick-access:not(.quick-access--open) .quick-access__toggle {
  width: 128px;
}
.quick-access__body {
  min-height: 0;
  overflow-y: auto;
  padding: 0 8px 8px;
}
.quick-access__items {
  display: grid;
  gap: 6px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin-top: 6px;
}
.quick-access__item {
  cursor: pointer;
  min-width: 0;
  min-height: 64px;
  padding: 10px !important;
  position: relative;
}
.quick-access__remove {
  height: 18px !important;
  position: absolute;
  right: 4px;
  top: 4px;
  width: 18px !important;
  z-index: 1;
}
</style>
