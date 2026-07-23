<template>
  <v-virtual-scroll
    :items="items"
    height="calc(100vh - 260px)"
    item-height="48"
    class="ma-2"
    style="user-select: none"
  >
    <template #default="{ item }">
      <div
        :key="item.code"
        :style="{
          transform: `translateX(${swipeOffset(item)}px)`,
          transition: swiping ? 'none' : 'transform 0.2s ease',
        }"
        @pointerdown="onPointerDown($event, item)"
        @pointermove="onPointerMove"
        @pointerup="onPointerUp"
        @pointercancel="onPointerCancel"
      >
        <v-list-item>
          <template #prepend>
            <v-avatar size="32" class="me-3 bg-grey-lighten-2">
              <v-icon :icon="iconFor(item.category)" size="18" />
            </v-avatar>
          </template>
          <template #title>
            <span class="text-caption font-weight-medium">{{ item.name }}</span>
          </template>
          <template #subtitle>
            <span class="text-overline text-medium-emphasis">{{
              item.code
            }}</span>
          </template>
          <template #append>
            <div class="d-flex align-center" style="min-width: 220px">
              <div
                class="d-flex align-center ga-1"
                style="width: 60px; flex: 0 0 auto"
              >
                <v-btn
                  density="compact"
                  variant="text"
                  size="small"
                  @click="decreaseQty(item)"
                >
                  <v-icon icon="mdi-minus" size="20" />
                </v-btn>
                <span
                  class="text-body-2 font-weight-bold text-center"
                  style="min-width: 1.5rem"
                  >{{ item.qty }}</span
                >
                <v-btn
                  density="compact"
                  variant="text"
                  size="small"
                  @click="increaseQty(item)"
                >
                  <v-icon icon="mdi-plus" size="20" />
                </v-btn>
              </div>
              <div class="d-flex flex-column align-end" style="flex: 1 1 auto">
                <span class="text-caption text-medium-emphasis"
                  >${{ item.price.toFixed(2) }}</span
                >
                <span v-if="item.discount" class="text-caption text-red"
                  >-{{ item.discount }}%</span
                >
                <span class="text-caption font-weight-bold"
                  >${{ totalAfterDiscount(item).toFixed(2) }}</span
                >
              </div>
            </div>
          </template>
        </v-list-item>
      </div>
    </template>
  </v-virtual-scroll>

  <v-dialog v-model="confirmDialog" max-width="400">
    <v-card>
      <v-card-title>Remove Item</v-card-title>
      <v-card-text
        >Remove <strong>{{ itemToRemove?.name }}</strong> from the
        invoice?</v-card-text
      >
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="cancelRemove">Cancel</v-btn>
        <v-btn variant="elevated" color="red-darken-4" @click="confirmRemove"
          >Remove</v-btn
        >
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
import { ref, computed, onMounted } from "vue";
import { ListCategoryIcons } from "../../wailsjs/go/main/App";

interface LineItem {
  name: string;
  code: string;
  category: string;
  price: number;
  discount: number;
  qty: number;
}

function discountAmount(item: LineItem): number {
  return item.price * item.qty * (item.discount / 100);
}

function totalAfterDiscount(item: LineItem): number {
  return item.price * item.qty - discountAmount(item);
}

const categoryIcons = ref<Map<string, string>>(new Map());

function iconFor(category: string): string {
  return categoryIcons.value.get(category) || "mdi-cart-outline";
}

const items = ref<LineItem[]>([]);

onMounted(async () => {
  try {
    const entries = (await ListCategoryIcons()) as { icon: string; categories: string }[];
    const map = new Map<string, string>();
    for (const row of entries) {
      for (const cat of row.categories.split(",")) {
        map.set(cat.trim(), row.icon);
      }
    }
    categoryIcons.value = map;
  } catch {}
});

function increaseQty(item: LineItem) {
  item.qty++;
}

function decreaseQty(item: LineItem) {
  if (item.qty > 1) item.qty--;
}

const confirmDialog = ref(false);
const itemToRemove = ref<LineItem | null>(null);
let pointerStartX = 0;
let swiping = ref(false);
const offsets = ref<Record<string, number>>({});

function swipeOffset(item: LineItem): number {
  return offsets.value[item.code] ?? 0;
}

function onPointerDown(event: PointerEvent, item: LineItem) {
  pointerStartX = event.clientX;
  itemToRemove.value = item;
  swiping.value = true;
  (event.target as HTMLElement).setPointerCapture(event.pointerId);
}

function onPointerMove(event: PointerEvent) {
  if (!swiping.value || !itemToRemove.value) return;
  const dx = event.clientX - pointerStartX;
  const offset = Math.max(-110, Math.min(0, dx));
  offsets.value[itemToRemove.value.code] = offset;
}

function onPointerUp(event: PointerEvent) {
  if (!itemToRemove.value) return;
  const dx = event.clientX - pointerStartX;
  if (dx < -60) {
    confirmDialog.value = true;
  } else {
    offsets.value[itemToRemove.value.code] = 0;
  }
  swiping.value = false;
}

function onPointerCancel() {
  if (itemToRemove.value) {
    offsets.value[itemToRemove.value.code] = 0;
  }
  swiping.value = false;
}

function confirmRemove() {
  if (itemToRemove.value) {
    items.value = items.value.filter(
      (i) => i.code !== itemToRemove.value!.code,
    );
  }
  confirmDialog.value = false;
  itemToRemove.value = null;
}

function cancelRemove() {
  if (itemToRemove.value) {
    offsets.value[itemToRemove.value.code] = 0;
  }
  confirmDialog.value = false;
  itemToRemove.value = null;
}

function addItem(item: LineItem) {
  const existing = items.value.find((i) => i.code === item.code);
  if (existing) {
    existing.qty++;
  } else {
    items.value.push({ ...item });
  }
}

function clearItems() {
  items.value = [];
}

const subtotal = computed(() =>
  items.value.reduce((sum, i) => sum + i.price * i.qty, 0),
);
const totalDiscount = computed(() =>
  items.value.reduce((sum, i) => sum + discountAmount(i), 0),
);

defineExpose({ subtotal, totalDiscount, addItem, items, clearItems });
</script>
