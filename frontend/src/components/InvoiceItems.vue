<template>
  <v-virtual-scroll :items="items" height="100%" item-height="48" class="ma-2">
    <template #default="{ item }">
      <v-list-item>
        <!-- <template #prepend> -->
        <!--   <v-avatar size="32" class="me-3 bg-grey-lighten-2"> -->
        <!--     <v-icon :icon="iconFor(item.category)" size="18" /> -->
        <!--   </v-avatar> -->
        <!-- </template> -->
        <template #title>
          <span class="text-caption font-weight-medium">{{ item.name }}</span>
        </template>
        <template #subtitle>
          <span class="text-overline text-medium-emphasis">{{ item.code }}</span>
        </template>
        <template #append>
          <div class="d-flex align-center" style="min-width: 240px">
            <div class="d-flex align-center ga-1" style="width: 60px; flex: 0 0 auto">
              <v-btn
                v-if="!readonly"
                density="compact"
                variant="text"
                size="small"
                @click="decreaseQty(item)"
              >
                <v-icon icon="mdi-minus" size="20" />
              </v-btn>
              <span class="text-body-2 font-weight-bold text-center" style="min-width: 1.5rem">{{
                item.qty
              }}</span>
              <v-btn
                v-if="!readonly"
                density="compact"
                variant="text"
                size="small"
                @click="increaseQty(item)"
              >
                <v-icon icon="mdi-plus" size="20" />
              </v-btn>
            </div>
            <div class="d-flex flex-column align-end" style="flex: 1 1 auto">
              <span class="text-caption text-medium-emphasis">${{ item.price.toFixed(2) }}</span>
              <span v-if="item.discount" class="text-caption text-red">-{{ item.discount }}%</span>
              <span class="text-caption font-weight-bold"
                >${{ totalAfterDiscount(item).toFixed(2) }}</span
              >
            </div>
            <v-btn v-if="!readonly" variant="text" rounded size="small" @click="removeItem(item)">
              <v-icon icon="mdi-delete" size="18" color="grey" />
            </v-btn>
          </div>
        </template>
      </v-list-item>
    </template>
  </v-virtual-scroll>
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

const props = defineProps<{ roundingMode?: string; readonly?: boolean }>();
const readonly = computed(() => props.readonly ?? false);

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

function removeItem(item: LineItem) {
  items.value = items.value.filter((i) => i.code !== item.code);
}

function addItem(item: LineItem) {
  const existing = items.value.find((i) => i.code === item.code);
  if (existing) {
    existing.qty++;
  } else {
    items.value.push({ ...item });
  }
}

function addExchangeItems(returned: LineItem, replacements: LineItem[]) {
  items.value = [
    { ...returned, price: Number(returned.price), discount: 0, qty: -1 },
    ...replacements.map((item) => ({
      ...item,
      price: Number(item.price),
      qty: Number(item.qty || 1),
    })),
  ];
}

function clearItems() {
  items.value = [];
}

const subtotal = computed(() => items.value.reduce((sum, i) => sum + i.price * i.qty, 0));
const totalDiscount = computed(() => items.value.reduce((sum, i) => sum + discountAmount(i), 0));
const subtotalCents = computed(() =>
  items.value.reduce((sum, i) => sum + Math.round(i.price * 100) * Math.trunc(i.qty), 0),
);
const discountedSubtotalCents = computed(() =>
  items.value.reduce((sum, i) => {
    const gross = Math.round(i.price * 100) * Math.trunc(i.qty);
    const discount = roundDiscount(gross, i.discount, props.roundingMode ?? "half_up");
    return sum + gross - discount;
  }, 0),
);

function roundDiscount(gross: number, discount: number, mode: string): number {
  const numerator = gross * Math.trunc(discount);
  if (mode === "down") return Math.floor(numerator / 100);
  if (mode === "up") return Math.ceil(numerator / 100);
  return Math.floor(numerator / 100 + 0.5);
}

defineExpose({
  subtotal,
  totalDiscount,
  subtotalCents,
  discountedSubtotalCents,
  addItem,
  addExchangeItems,
  items,
  clearItems,
});
</script>
