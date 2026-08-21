<template>
  <div ref="invoiceScroll" class="invoice-items ms-2 mt-2 mb-2 me-0">
    <div v-for="(item, index) in items" :key="item.barcode || item.code">
      <v-list-item>
        <template #title>
          <button
            class="invoice-item-details-trigger"
            type="button"
            :aria-label="`Open details for ${item.name}`"
            @click.stop="openDetails(item)"
          >
            <span class="invoice-item-name text-caption font-weight-medium">{{ item.name }}</span>
            <span class="invoice-item-code text-overline text-medium-emphasis">
              {{ item.code }}
              <span v-if="item.color || item.size" class="text-caption text-medium-emphasis ms-2">
                {{ [item.color, item.size].filter(Boolean).join(" / ") }}
              </span>
            </span>
          </button>
        </template>
        <template #append>
          <div class="invoice-item-actions d-flex align-center">
            <div class="invoice-qty-control">
              <NumberPadMenu
                :model-value="item.qty"
                :disabled="readonly || item.qty < 0"
                :min="1"
                :max="999"
                aria-label="Edit item quantity"
                prefix="Qty"
                @update:model-value="(value) => setQuantity(item, Number(value))"
              />
            </div>
            <div class="invoice-discount-control">
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
            </div>
            <NumberPadMenu
              v-if="customPricesEnabled && !readonly && item.qty > 0"
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
                  class="invoice-price-editor invoice-price-control d-flex flex-column align-end"
                  type="button"
                  aria-label="Set custom item price"
                  title="Set custom item price"
                >
                  <span class="text-caption text-medium-emphasis">
                    ${{ item.price.toFixed(2) }}
                    <v-icon
                      v-if="item.priceListOverridden"
                      icon="mdi-star"
                      color="amber-darken-2"
                      size="12"
                      :title="item.priceListName ? `Price from list: ${item.priceListName}` : 'List price'"
                    />
                  </span>
                  <span class="text-caption font-weight-bold">
                    ${{ totalAfterDiscount(item).toFixed(2) }}
                  </span>
                </button>
              </template>
            </NumberPadMenu>
            <div v-else class="invoice-price-control d-flex flex-column align-end">
              <span class="text-caption text-medium-emphasis">
                ${{ item.price.toFixed(2) }}
                <v-icon
                  v-if="item.priceListOverridden"
                  icon="mdi-star"
                  color="amber-darken-2"
                  size="12"
                  :title="item.priceListName ? `Price from list: ${item.priceListName}` : 'List price'"
                />
              </span>
              <span v-if="item.discount && !customDiscountsEnabled" class="text-caption text-red">
                -{{ item.discount }}%
                <v-icon
                  v-if="item.discountListOverridden"
                  icon="mdi-star"
                  color="amber-darken-2"
                  size="12"
                  :title="item.discountListName ? `Discount from list: ${item.discountListName}` : 'List discount'"
                />
              </span>
              <span class="text-caption font-weight-bold"
                >${{ totalAfterDiscount(item).toFixed(2) }}</span
              >
            </div>
            <div class="invoice-delete-control">
              <v-btn v-if="!readonly" variant="text" rounded size="small" @click="removeItem(item)">
                <v-icon icon="mdi-delete" size="18" color="grey" />
              </v-btn>
            </div>
          </div>
        </template>
      </v-list-item>
    </div>

    <v-dialog v-model="detailsDialog" width="600" height="500" @after-leave="detailItem = null">
      <v-card v-if="detailItem">
        <v-card-title class="d-flex align-center">
          <div class="min-width-0">
            <div class="text-subtitle-1 text-truncate">{{ detailItem.name }}</div>
            <div class="text-caption text-medium-emphasis">{{ detailItem.code }}</div>
          </div>
          <v-spacer />
          <v-btn
            icon="mdi-close"
            variant="text"
            aria-label="Close item details"
            @click="detailsDialog = false"
          />
        </v-card-title>
        <v-card-text class="invoice-item-details-content">
          <v-select
            :model-value="detailItem.salespersonId || 0"
            :items="salespersonOptions"
            item-title="title"
            item-value="value"
            label="Salesperson for this line"
            prepend-inner-icon="mdi-account-tie"
            variant="outlined"
            density="compact"
            :menu-props="{ transition: animationsEnabled ? 'fade-transition' : false }"
            @update:model-value="detailItem.salespersonId = Number($event || 0)"
          />
          <v-textarea
            v-model="detailItem.note"
            placeholder="Add a note about this item"
            prepend-inner-icon="mdi-note-text-outline"
            variant="outlined"
            density="compact"
            auto-grow
            rows="2"
            maxlength="2000"
            counter
          />
          <div class="text-subtitle-2 mb-2">Units</div>
          <div class="invoice-unit-lines">
            <div
              v-for="unitIndex in detailUnitCount(detailItem)"
              :key="unitIndex"
              class="invoice-unit-line d-flex align-center ga-2"
            >
              <span class="text-caption text-medium-emphasis">Unit {{ unitIndex }}</span>
              <v-spacer />
              <NumberPadMenu
                v-if="canEditUnitDiscount(detailItem)"
                :model-value="unitDiscount(detailItem, unitIndex - 1)"
                :aria-label="`Set discount for unit ${unitIndex}`"
                :min="0"
                :max="100"
                suffix="%"
                tone="error"
                @update:model-value="(value) => setUnitDiscount(detailItem!, unitIndex - 1, value)"
              />
              <span v-else class="text-caption text-red me-3">
                -{{ unitDiscount(detailItem, unitIndex - 1) }}%
              </span>
              <NumberPadMenu
                v-if="canEditUnitPrice(detailItem)"
                :model-value="unitPrice(detailItem, unitIndex - 1).toFixed(2)"
                allow-decimal
                :aria-label="`Set price for unit ${unitIndex}`"
                :min="0.01"
                :max="999999.99"
                :max-length="9"
                prefix="$"
                @update:model-value="(value) => setUnitPrice(detailItem!, unitIndex - 1, value)"
              />
              <span v-else class="text-body-2 font-weight-medium">
                ${{ unitPrice(detailItem, unitIndex - 1).toFixed(2) }}
              </span>
            </div>
          </div>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn color="primary" variant="flat" @click="detailsDialog = false">Done</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script lang="ts" setup>
import {
  computed,
  inject,
  nextTick,
  onMounted,
  ref,
  watch,
  type ComponentPublicInstance,
  type Ref,
} from "vue";
import {
  ListCategoryIcons,
  ListSalespersons,
  UpdateProductDiscount,
} from "../../wailsjs/go/main/App";
import { roundDiscountCents, normalizeRoundingMode } from "@/lib/money";
import { animationsEnabled } from "@/lib/display";
import NumberPadMenu from "@/components/NumberPadMenu.vue";
import type { Salesperson } from "@/composables/useSalesperson";

interface LineItem {
  name: string;
  code: string;
  barcode?: string;
  color?: string;
  size?: string;
  category: string;
  price: number;
  soldUnitPrice?: number;
  soldUnitDiscount?: number;
  unitPrices?: number[];
  unitDiscounts?: number[];
  discount: number;
  qty: number;
  salespersonId?: number;
  note?: string;
  priceListOverridden?: boolean;
  priceListName?: string;
  discountListOverridden?: boolean;
  discountListName?: string;
}

const emit = defineEmits<{
  "discount-error": [message: string];
  "discount-saved": [code: string, discount: number];
}>();
const props = defineProps<{
  roundingMode?: string;
  readonly?: boolean;
  customDiscountsEnabled?: boolean;
  customPricesEnabled?: boolean;
  perUnitPricesEnabled?: boolean;
}>();
const readonly = computed(() => props.readonly ?? false);
const customDiscountsEnabled = computed(() => props.customDiscountsEnabled ?? false);
const customPricesEnabled = computed(() => props.customPricesEnabled ?? false);
const perUnitPricesEnabled = computed(() => props.perUnitPricesEnabled ?? false);

function discountAmount(item: LineItem): number {
  return (itemGrossCents(item) - discountedItemCents(item)) / 100;
}

function totalAfterDiscount(item: LineItem): number {
  return discountedItemCents(item) / 100;
}

function itemGrossCents(item: LineItem): number {
  const qty = Math.max(0, Math.trunc(item.qty));
  if (item.qty < 0) return Math.round(item.price * 100) * Math.trunc(item.qty);
  ensureUnitPrices(item);
  return (item.unitPrices ?? [])
    .slice(0, qty)
    .reduce((sum, price) => sum + Math.round(price * 100), 0);
}

function itemGross(item: LineItem): number {
  return itemGrossCents(item) / 100;
}

function discountedItemCents(item: LineItem): number {
  if (item.qty < 0) {
    const gross = itemGrossCents(item);
    return (
      gross -
      roundDiscountCents(gross, item.discount, normalizeRoundingMode(props.roundingMode ?? ""))
    );
  }
  ensureUnitPrices(item);
  const groups = new Map<string, { gross: number; discount: number }>();
  for (let index = 0; index < Math.trunc(item.qty); index++) {
    const price = unitPrice(item, index);
    const discount = unitDiscount(item, index);
    const cents = Math.round(price * 100);
    const key = `${cents}:${discount}`;
    const group = groups.get(key) ?? { gross: 0, discount };
    group.gross += cents;
    groups.set(key, group);
  }
  let total = 0;
  for (const group of groups.values()) {
    total +=
      group.gross -
      roundDiscountCents(
        group.gross,
        group.discount,
        normalizeRoundingMode(props.roundingMode ?? ""),
      );
  }
  return total;
}

const categoryIcons = ref<Map<string, string>>(new Map());
const salespersons = ref<Salesperson[]>([]);
const salespersonOptions = computed(() => [
  { title: "Invoice salesperson (default)", value: 0 },
  ...salespersons.value.map((person) => ({ title: person.fullName, value: person.id })),
]);

function iconFor(category: string): string {
  return categoryIcons.value.get(category) || "mdi-cart-outline";
}

const items = ref<LineItem[]>([]);
const savedDiscounts = new Map<string, number>();
const pendingDiscounts = new Map<string, Promise<boolean>>();
const discountInputs = new Map<number, { focus: () => void }>();
const invoiceScroll = ref<HTMLElement | null>(null);
const detailsDialog = ref(false);
const detailItem = ref<LineItem | null>(null);
const dialogOpen = inject<Ref<boolean> | null>("dialogOpen", null);

watch(detailsDialog, (open) => {
  if (dialogOpen) dialogOpen.value = open;
});

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
  try {
    salespersons.value = ((await ListSalespersons()) as Salesperson[]).sort((a, b) =>
      a.fullName.localeCompare(b.fullName, undefined, { sensitivity: "base" }),
    );
  } catch {}
});

function setQuantity(item: LineItem, value: number) {
  const qty = Math.max(1, Math.trunc(value));
  ensureUnitPrices(item);
  while ((item.unitPrices?.length ?? 0) < qty) item.unitPrices?.push(item.price);
  while ((item.unitDiscounts?.length ?? 0) < qty) item.unitDiscounts?.push(item.discount);
  item.unitPrices = item.unitPrices?.slice(0, qty);
  item.unitDiscounts = item.unitDiscounts?.slice(0, qty);
  item.qty = qty;
}

function removeItem(item: LineItem) {
  items.value = items.value.filter((i) => i.barcode !== item.barcode);
}

function addItem(item: LineItem) {
  const existing = items.value.find(
    (i) => (item.barcode && i.barcode === item.barcode) || (!item.barcode && i.code === item.code),
  );
  if (existing) {
    ensureUnitPrices(existing);
    existing.unitPrices?.push(existing.price);
    existing.unitDiscounts?.push(existing.discount);
    existing.qty++;
  } else {
    const discount = savedDiscounts.get(item.code) ?? Number(item.discount || 0);
    savedDiscounts.set(item.code, discount);
    const price = Number(item.price);
    const qty = Math.max(1, Math.trunc(Number(item.qty || 1)));
    const unitPrices =
      Array.isArray(item.unitPrices) && item.unitPrices.length === qty
        ? item.unitPrices.map(Number)
        : Array(qty).fill(price);
    const unitDiscounts =
      Array.isArray(item.unitDiscounts) && item.unitDiscounts.length === qty
        ? item.unitDiscounts.map(Number)
        : Array(qty).fill(discount);
    items.value.push({
      ...item,
      discount,
      price,
      qty,
      note: item.note ?? "",
      unitPrices,
      unitDiscounts,
    });
  }
}

function addExchangeItems(returned: LineItem[], replacements: LineItem[]) {
  items.value = [
    ...returned.map((item) => ({
      ...item,
      price: Number(item.price),
      discount: 0,
      qty: -Number(item.qty || 1),
      soldUnitPrice: Number(item.soldUnitPrice ?? item.price),
      soldUnitDiscount: Number(item.soldUnitDiscount ?? item.discount),
      unitPrices: [],
      unitDiscounts: [],
    })),
    ...replacements.map((item) => ({
      ...item,
      price: Number(item.price),
      qty: Number(item.qty || 1),
      unitPrices: Array(Math.max(1, Math.trunc(Number(item.qty || 1)))).fill(Number(item.price)),
      unitDiscounts: Array(Math.max(1, Math.trunc(Number(item.qty || 1)))).fill(
        Number(item.discount || 0),
      ),
    })),
  ];
}

function clearItems() {
  items.value = [];
  detailsDialog.value = false;
  detailItem.value = null;
}

function canEditUnitPrice(item: LineItem): boolean {
  return (
    item.qty > 0 && (perUnitPricesEnabled.value || customPricesEnabled.value) && !readonly.value
  );
}

function canEditUnitDiscount(item: LineItem): boolean {
  return (
    item.qty > 0 && (perUnitPricesEnabled.value || customDiscountsEnabled.value) && !readonly.value
  );
}

function openDetails(item: LineItem) {
  ensureUnitPrices(item);
  item.note ??= "";
  detailItem.value = item;
  detailsDialog.value = true;
}

function detailUnitCount(item: LineItem): number {
  return Math.max(1, Math.abs(Math.trunc(item.qty)));
}

function ensureUnitPrices(item: LineItem) {
  if (item.qty < 0) return;
  const qty = Math.max(1, Math.trunc(item.qty));
  if (!Array.isArray(item.unitPrices)) item.unitPrices = [];
  if (!Array.isArray(item.unitDiscounts)) item.unitDiscounts = [];
  while (item.unitPrices.length < qty) item.unitPrices.push(item.price);
  while (item.unitDiscounts.length < qty) item.unitDiscounts.push(item.discount);
  if (item.unitPrices.length > qty) item.unitPrices = item.unitPrices.slice(0, qty);
  if (item.unitDiscounts.length > qty) item.unitDiscounts = item.unitDiscounts.slice(0, qty);
}

function unitPrice(item: LineItem, index: number): number {
  if (item.qty < 0) return Number(item.soldUnitPrice ?? item.price);
  ensureUnitPrices(item);
  return Number(item.unitPrices?.[index] ?? item.price);
}

function unitDiscount(item: LineItem, index: number): number {
  if (item.qty < 0) return Number(item.soldUnitDiscount ?? item.discount);
  ensureUnitPrices(item);
  return Number(item.unitDiscounts?.[index] ?? item.discount);
}

function setDiscountInput(index: number, element: Element | ComponentPublicInstance | null) {
  if (element && "focus" in element && typeof element.focus === "function") {
    discountInputs.set(index, element as { focus: () => void });
  } else {
    discountInputs.delete(index);
  }
}

function setDraftDiscount(item: LineItem, value: unknown) {
  const numeric = Number(value);
  item.discount = Number.isFinite(numeric) ? Math.min(100, Math.max(0, Math.trunc(numeric))) : 0;
  if (item.qty > 0) item.unitDiscounts = Array(Math.trunc(item.qty)).fill(item.discount);
  // A manual edit always wins over a discount-list default, so the star
  // (which means "this came from a list") no longer applies.
  item.discountListOverridden = false;
}

function setDraftPrice(item: LineItem, value: unknown) {
  const numeric = Number(value);
  item.price = Number.isFinite(numeric)
    ? Math.round(Math.min(999999.99, Math.max(0.01, numeric)) * 100) / 100
    : 0.01;
  if (item.qty > 0) item.unitPrices = Array(Math.trunc(item.qty)).fill(item.price);
  item.priceListOverridden = false;
}

function setUnitPrice(item: LineItem, index: number, value: unknown) {
  const numeric = Number(value);
  const price = Number.isFinite(numeric)
    ? Math.round(Math.min(999999.99, Math.max(0.01, numeric)) * 100) / 100
    : 0.01;
  ensureUnitPrices(item);
  if (item.unitPrices) item.unitPrices[index] = price;
}

function setUnitDiscount(item: LineItem, index: number, value: unknown) {
  const numeric = Number(value);
  const discount = Number.isFinite(numeric) ? Math.min(100, Math.max(0, Math.trunc(numeric))) : 0;
  ensureUnitPrices(item);
  if (item.unitDiscounts) item.unitDiscounts[index] = discount;
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
      if (item.qty > 0) item.unitDiscounts = Array(Math.trunc(item.qty)).fill(previous);
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

  invoiceScroll.value?.children[target]?.scrollIntoView({ block: "nearest" });
  await nextTick();
  requestAnimationFrame(() => discountInputs.get(target)?.focus());
}

async function flushChanges(): Promise<boolean> {
  const discountResults = await Promise.all(items.value.map(persistDiscount));
  return discountResults.every(Boolean);
}

const subtotal = computed(() => items.value.reduce((sum, i) => sum + itemGross(i), 0));
const totalDiscount = computed(() => items.value.reduce((sum, i) => sum + discountAmount(i), 0));
const subtotalCents = computed(() => items.value.reduce((sum, i) => sum + itemGrossCents(i), 0));
const discountedSubtotalCents = computed(() =>
  items.value.reduce((sum, i) => sum + discountedItemCents(i), 0),
);

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
  justify-content: flex-end;
  margin-inline-start: auto;
  white-space: nowrap;
}

.invoice-qty-control,
.invoice-discount-control,
.invoice-delete-control {
  display: flex;
  flex: 0 0 auto;
  justify-content: center;
}

.invoice-qty-control {
  width: 56px;
}

.invoice-discount-control {
  width: 44px;
}

.invoice-delete-control {
  width: 40px;
}

.invoice-items :deep(.v-list-item__content) {
  min-width: 0;
}

.invoice-items :deep(.v-list-item__append) {
  justify-content: flex-end;
  margin-inline-start: auto;
}

.invoice-items {
  height: 100%;
  overflow-y: auto;
}

.invoice-item-details-trigger {
  background: transparent;
  border: 0;
  border-radius: 4px;
  color: inherit;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  margin-inline-start: -6px;
  max-width: 100%;
  overflow: hidden;
  padding: 3px 6px;
  text-align: start;
  width: calc(100% + 6px);
}

.invoice-item-name,
.invoice-item-code {
  display: block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.invoice-item-details-trigger:hover,
.invoice-item-details-trigger:focus-visible {
  background: rgba(var(--v-theme-primary), 0.1);
}

.invoice-item-details-trigger:focus-visible {
  outline: 2px solid rgba(var(--v-theme-primary), 0.45);
  outline-offset: 1px;
}

.invoice-unit-lines {
  background: rgba(var(--v-theme-on-surface), 0.035);
  border-inline-start: 2px solid rgba(var(--v-theme-primary), 0.3);
  max-height: 280px;
  overflow-y: auto;
  padding: 6px 12px;
}

.invoice-unit-line {
  min-height: 38px;
}

.invoice-item-details-content {
  overflow-y: auto;
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

.invoice-price-control {
  flex: 0 0 72px;
  width: 72px;
}

.invoice-price-editor:hover,
.invoice-price-editor:focus-visible {
  background: rgba(var(--v-theme-primary), 0.09);
  outline: none;
}
</style>
