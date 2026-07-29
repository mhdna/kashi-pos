<template>
  <v-virtual-scroll
    ref="invoiceScroll"
    :items="items"
    height="100%"
    item-height="56"
    class="ma-2"
  >
    <template #default="{ item, index }">
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
          <span v-if="item.color || item.size" class="text-caption text-medium-emphasis ms-2">
            {{ [item.color, item.size].filter(Boolean).join(" / ") }}
          </span>
        </template>
        <template #append>
          <div class="d-flex align-center ga-2" style="min-width: 320px">
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
            <v-text-field
              v-if="customDiscountsEnabled && !readonly && item.qty > 0"
              :ref="(element) => setDiscountInput(index, element)"
              :model-value="item.discount"
              aria-label="Item discount percentage"
              class="invoice-discount-input"
              density="compact"
              hide-details
              max="100"
              min="0"
              step="1"
              suffix="%"
              type="number"
              variant="outlined"
              @focus="selectDiscount"
              @update:model-value="(value) => setDraftDiscount(item, value)"
              @blur="persistDiscount(item)"
              @keydown.enter.prevent="saveAndMove(item, index, 1)"
              @keydown.up.prevent="saveAndMove(item, index, -1)"
              @keydown.down.prevent="saveAndMove(item, index, 1)"
            />
            <div class="d-flex flex-column align-end" style="flex: 1 1 auto">
              <span class="text-caption text-medium-emphasis">${{ item.price.toFixed(2) }}</span>
              <span
                v-if="item.discount && !customDiscountsEnabled"
                class="text-caption text-red"
                >-{{ item.discount }}%</span
              >
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
import {
  computed,
  nextTick,
  onMounted,
  ref,
  type ComponentPublicInstance,
} from "vue";
import { ListCategoryIcons, UpdateProductDiscount } from "../../wailsjs/go/main/App";

interface LineItem {
  name: string;
  code: string;
  barcode?: string;
  color?: string;
  size?: string;
  category: string;
  price: number;
  discount: number;
  qty: number;
}

const emit = defineEmits<{
  "discount-error": [message: string];
  "discount-saved": [code: string, discount: number];
}>();
const props = defineProps<{
  roundingMode?: string;
  readonly?: boolean;
  customDiscountsEnabled?: boolean;
}>();
const readonly = computed(() => props.readonly ?? false);
const customDiscountsEnabled = computed(() => props.customDiscountsEnabled ?? false);

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
const savedDiscounts = new Map<string, number>();
const pendingDiscounts = new Map<string, Promise<boolean>>();
const discountInputs = new Map<number, { focus: () => void }>();
const invoiceScroll = ref<{ scrollToIndex?: (index: number) => void } | null>(null);

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
  items.value = items.value.filter((i) => i.barcode !== item.barcode);
}

function addItem(item: LineItem) {
  const existing = items.value.find((i) => i.barcode === item.barcode);
  if (existing) {
    existing.qty++;
  } else {
    const discount = savedDiscounts.get(item.code) ?? Number(item.discount || 0);
    savedDiscounts.set(item.code, discount);
    items.value.push({ ...item, discount });
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

function setDiscountInput(
  index: number,
  element: Element | ComponentPublicInstance | null,
) {
  if (element && "focus" in element && typeof element.focus === "function") {
    discountInputs.set(index, element as { focus: () => void });
  } else {
    discountInputs.delete(index);
  }
}

function selectDiscount(event: FocusEvent) {
  (event.target as HTMLInputElement | null)?.select();
}

function setDraftDiscount(item: LineItem, value: unknown) {
  const numeric = Number(value);
  item.discount = Number.isFinite(numeric)
    ? Math.min(100, Math.max(0, Math.trunc(numeric)))
    : 0;
}

function persistDiscount(item: LineItem): Promise<boolean> {
  const pending = pendingDiscounts.get(item.code);
  if (pending) return pending;

  const discount = Math.min(100, Math.max(0, Math.trunc(Number(item.discount) || 0)));
  item.discount = discount;
  const previous = savedDiscounts.get(item.code) ?? discount;
  if (discount === previous) return Promise.resolve(true);

  const request = UpdateProductDiscount(item.code, discount)
    .then(() => {
      savedDiscounts.set(item.code, discount);
      emit("discount-saved", item.code, discount);
      return true;
    })
    .catch((error: unknown) => {
      item.discount = previous;
      emit(
        "discount-error",
        error instanceof Error ? error.message : String(error || "Could not save discount."),
      );
      return false;
    })
    .finally(() => pendingDiscounts.delete(item.code));
  pendingDiscounts.set(item.code, request);
  return request;
}

async function saveAndMove(item: LineItem, index: number, direction: -1 | 1) {
  await persistDiscount(item);
  const target = index + direction;
  if (target < 0 || target >= items.value.length) return;

  invoiceScroll.value?.scrollToIndex?.(target);
  await nextTick();
  requestAnimationFrame(() => discountInputs.get(target)?.focus());
}

async function flushDiscounts(): Promise<boolean> {
  const results = await Promise.all(items.value.map(persistDiscount));
  return results.every(Boolean);
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
  flushDiscounts,
});
</script>

<style scoped>
.invoice-discount-input {
  flex: 0 0 78px;
  max-width: 78px;
}
</style>
