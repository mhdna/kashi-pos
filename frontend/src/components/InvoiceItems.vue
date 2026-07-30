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
          <div class="invoice-item-actions d-flex align-center ga-1">
            <NumberPadMenu
              :model-value="item.qty"
              :disabled="readonly || item.qty < 0"
              :min="1"
              :max="999"
              aria-label="Edit item quantity"
              prefix="Qty"
              @update:model-value="(value) => setQuantity(item, Number(value))"
            />
            <NumberPadMenu
              v-if="customDiscountsEnabled && !readonly && item.qty > 0"
              :ref="(element) => setDiscountInput(index, element)"
              :model-value="item.discount"
              aria-label="Item discount percentage"
              :min="0"
              :max="100"
              suffix="%"
              tone="error"
              @update:model-value="(value) => setDraftDiscount(item, value)"
              @commit="persistDiscount(item)"
              @navigate="(direction) => saveAndMove(item, index, direction)"
            />
            <NumberPadMenu
              v-if="customDiscountsEnabled && !readonly && item.qty > 0"
              :model-value="item.price.toFixed(2)"
              allow-decimal
              aria-label="Set custom item price"
              :min="0.01"
              :max="999999.99"
              :max-length="9"
              prefix="$"
              @update:model-value="(value) => setDraftPrice(item, value)"
            >
              <template #activator="{ props: activatorProps }">
                <button
                  v-bind="activatorProps"
                  class="invoice-price-editor d-flex flex-column align-end"
                  type="button"
                  aria-label="Set custom item price"
                  title="Set custom item price"
                >
                  <span class="text-caption text-medium-emphasis">
                    ${{ item.price.toFixed(2) }}
                  </span>
                  <span class="text-caption font-weight-bold">
                    ${{ totalAfterDiscount(item).toFixed(2) }}
                  </span>
                </button>
              </template>
            </NumberPadMenu>
            <div v-else class="d-flex flex-column align-end" style="flex: 1 1 auto">
              <span class="text-caption text-medium-emphasis">
                ${{ item.price.toFixed(2) }}
              </span>
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
import NumberPadMenu from "@/components/NumberPadMenu.vue";

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

function setQuantity(item: LineItem, value: number) {
  item.qty = Math.max(1, Math.trunc(value));
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
    const price = Number(item.price);
    items.value.push({ ...item, discount, price });
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

function setDraftDiscount(item: LineItem, value: unknown) {
  const numeric = Number(value);
  item.discount = Number.isFinite(numeric)
    ? Math.min(100, Math.max(0, Math.trunc(numeric)))
    : 0;
}

function setDraftPrice(item: LineItem, value: unknown) {
  const numeric = Number(value);
  item.price = Number.isFinite(numeric)
    ? Math.round(Math.min(999999.99, Math.max(0.01, numeric)) * 100) / 100
    : 0.01;
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

async function flushChanges(): Promise<boolean> {
  const discountResults = await Promise.all(items.value.map(persistDiscount));
  return discountResults.every(Boolean);
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
  flushChanges,
});
</script>

<style scoped>
.invoice-item-actions {
  min-width: 280px;
}

.invoice-price-editor {
  background: transparent;
  border: 0;
  border-radius: 6px;
  color: inherit;
  cursor: pointer;
  flex: 1 1 auto;
  min-height: 36px;
  padding: 2px 5px;
}

.invoice-price-editor:hover,
.invoice-price-editor:focus-visible {
  background: rgba(var(--v-theme-primary), 0.09);
  outline: none;
}
</style>
