<template>
  <div ref="invoiceScroll" class="invoice-items ms-2 mt-2 mb-2 me-0">
    <div
      v-for="(item, index) in displayItems"
      :key="`${items.length - 1 - index}-${item.barcode || item.code}`"
    >
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
                :ref="(element) => setFieldRef(index, 'qty', element)"
                :model-value="item.qty"
                :disabled="readonly || item.locked || item.qty < 0 || isUnknownItem(item)"
                :min="1"
                :max="999"
                aria-label="Edit item quantity"
                prefix="Qty"
                :keyboard-mode="keyboardMode"
                @update:model-value="(value) => setQuantity(item, Number(value))"
                @navigate="(direction) => moveVertical(index, 'qty', direction)"
                @navigate-h="(direction) => moveHorizontal(index, 'qty', direction)"
              />
            </div>
            <div class="invoice-discount-control">
              <NumberPadMenu
                v-if="canEditDiscount(item)"
                :ref="(element) => setFieldRef(index, 'discount', element)"
                :model-value="item.discount"
                aria-label="Item discount percentage"
                :min="0"
                :max="100"
                suffix="%"
                tone="error"
                :keyboard-mode="keyboardMode"
                @update:model-value="(value) => setDraftDiscount(item, value)"
                @commit="persistDiscount(item)"
                @navigate="(direction) => moveFromDiscount(item, index, direction)"
                @navigate-h="(direction) => moveFromDiscountH(item, index, direction)"
              />
            </div>
            <NumberPadMenu
              v-if="canEditPrice(item)"
              :ref="(element) => setFieldRef(index, 'price', element)"
              :model-value="
                (priceChangeMode === 'discount' ? effectiveUnitPrice(item) : item.price).toFixed(2)
              "
              allow-decimal
              aria-label="Set custom item price"
              :min="0.01"
              :max="999999.99"
              :max-length="9"
              prefix="$"
              :keyboard-mode="keyboardMode"
              @update:model-value="(value) => setDraftPrice(item, value)"
              @navigate="(direction) => moveVertical(index, 'price', direction)"
              @navigate-h="(direction) => moveHorizontal(index, 'price', direction)"
            >
              <template
                #activator="{
                  props: activatorProps,
                  setRef,
                  onKeydown,
                  onFocus,
                  onBlur,
                  onClick,
                  keyboardMode: fieldKeyboardMode,
                }"
              >
                <button
                  v-bind="activatorProps"
                  :ref="setRef"
                  class="invoice-price-editor invoice-price-control d-flex flex-column align-end"
                  :class="{ 'invoice-price-editor--keyboard': fieldKeyboardMode }"
                  type="button"
                  aria-label="Set custom item price"
                  title="Set custom item price"
                  @click="onClick"
                  @keydown="onKeydown"
                  @focus="onFocus"
                  @blur="onBlur"
                >
                  <span class="invoice-price-value">
                    <v-icon
                      v-if="item.priceListOverridden"
                      icon="mdi-star"
                      color="amber-darken-2"
                      size="20"
                      :title="item.priceListName ? `Price from list: ${item.priceListName}` : 'List price'"
                    />
                    <span class="invoice-price-values">
                      <span class="text-caption text-medium-emphasis">${{ item.price.toFixed(2) }}</span>
                      <span class="text-caption font-weight-bold">${{ totalAfterDiscount(item).toFixed(2) }}</span>
                    </span>
                  </span>
                </button>
              </template>
            </NumberPadMenu>
            <div v-else class="invoice-price-control d-flex flex-column align-end">
              <span class="invoice-price-value">
                <v-icon
                  v-if="item.priceListOverridden"
                  icon="mdi-star"
                  color="amber-darken-2"
                  size="20"
                  :title="item.priceListName ? `Price from list: ${item.priceListName}` : 'List price'"
                />
                <span class="invoice-price-values">
                  <span class="text-caption text-medium-emphasis">${{ item.price.toFixed(2) }}</span>
                  <span v-if="item.discount && (!customDiscountsEnabled || !(isUnknownItem(item) || codeAllowsPriceEdit(item)))" class="text-caption text-red">
                    -{{ item.discount }}%
                    <v-icon
                      v-if="item.discountListOverridden"
                      icon="mdi-star"
                      color="amber-darken-2"
                      size="12"
                      :title="item.discountListName ? `Discount from list: ${item.discountListName}` : 'List discount'"
                    />
                  </span>
                  <span class="text-caption font-weight-bold">${{ totalAfterDiscount(item).toFixed(2) }}</span>
                </span>
              </span>
            </div>
            <div class="invoice-delete-control">
              <v-btn v-if="!readonly && !item.locked" variant="text" rounded size="small" @click="removeItem(item)">
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
            placeholder="Salesperson for this line"
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
                :ref="(element) => setUnitFieldRef(unitIndex - 1, 'discount', element)"
                :model-value="unitDiscount(detailItem, unitIndex - 1)"
                :aria-label="`Set discount for unit ${unitIndex}`"
                :min="0"
                :max="100"
                suffix="%"
                tone="error"
                :keyboard-mode="keyboardMode"
                @update:model-value="(value) => setUnitDiscount(detailItem!, unitIndex - 1, value)"
                @navigate="(direction) => moveUnit(unitIndex - 1, 'discount', direction)"
                @navigate-h="(direction) => moveUnitH(unitIndex - 1, 'discount', direction)"
              />
              <span v-else class="text-caption text-red me-3">
                -{{ unitDiscount(detailItem, unitIndex - 1) }}%
              </span>
              <NumberPadMenu
                v-if="canEditUnitPrice(detailItem)"
                :ref="(element) => setUnitFieldRef(unitIndex - 1, 'price', element)"
                :model-value="
                  (priceChangeMode === 'discount'
                    ? effectiveUnitPriceAt(detailItem, unitIndex - 1)
                    : unitPrice(detailItem, unitIndex - 1)
                  ).toFixed(2)
                "
                allow-decimal
                :aria-label="`Set price for unit ${unitIndex}`"
                :min="0.01"
                :max="999999.99"
                :max-length="9"
                prefix="$"
                :keyboard-mode="keyboardMode"
                @update:model-value="(value) => setUnitPrice(detailItem!, unitIndex - 1, value)"
                @navigate="(direction) => moveUnit(unitIndex - 1, 'price', direction)"
                @navigate-h="(direction) => moveUnitH(unitIndex - 1, 'price', direction)"
              />
              <span v-else class="text-body-2 font-weight-medium">
                ${{
                  (priceChangeMode === "discount"
                    ? effectiveUnitPriceAt(detailItem, unitIndex - 1)
                    : unitPrice(detailItem, unitIndex - 1)
                  ).toFixed(2)
                }}
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
  // The catalog/list price captured when the line was added, before any
  // manual edit. Used as the fixed reference point in "discount" price
  // change mode, where a price edit is expressed as a discount off this
  // value rather than as a new base price.
  originalPrice?: number;
  // Locks just this line, independent of the component-level `readonly`
  // prop - used by exchange mode, where returned (negative-qty) lines stay
  // locked while replacement lines scanned afterward are fully editable.
  locked?: boolean;
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
  // When restricted, price/discount are editable only on lines whose code is
  // in priceEditCodes (lowercased by the caller).
  priceEditRestricted?: boolean;
  priceEditCodes?: string[];
  perUnitPricesEnabled?: boolean;
  // "discount" (default): editing price or discount recomputes the other
  // from the line's original price, so a price change is always expressed
  // as a discount. "override": price and discount are edited independently.
  priceChangeMode?: "override" | "discount";
  // When true, clicking a line's qty/discount/price focuses it instead of
  // opening the number-pad popup, and arrow keys move focus between fields.
  keyboardMode?: boolean;
}>();
const readonly = computed(() => props.readonly ?? false);
const customDiscountsEnabled = computed(() => props.customDiscountsEnabled ?? false);
const customPricesEnabled = computed(() => props.customPricesEnabled ?? false);
const perUnitPricesEnabled = computed(() => props.perUnitPricesEnabled ?? false);
const priceChangeMode = computed(() => props.priceChangeMode ?? "discount");
const keyboardMode = computed(() => props.keyboardMode ?? false);

function effectiveUnitPrice(item: LineItem): number {
  const discount = Math.min(100, Math.max(0, Number(item.discount) || 0));
  return Math.round(item.price * (1 - discount / 100) * 100) / 100;
}

function effectiveUnitPriceAt(item: LineItem, index: number): number {
  const price = unitPrice(item, index);
  const discount = Math.min(100, Math.max(0, Number(unitDiscount(item, index)) || 0));
  return Math.round(price * (1 - discount / 100) * 100) / 100;
}

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
// Newest line on top. `items` itself stays in scan order (that's what the
// receipt, holds and the backend see); only the on-screen list is reversed,
// so row indexes in the template and keyboard navigation are display indexes.
const displayItems = computed(() => [...items.value].reverse());
const savedDiscounts = new Map<string, number>();
const pendingDiscounts = new Map<string, Promise<boolean>>();
type FieldColumn = "qty" | "discount" | "price";
const fieldRefs = new Map<string, { focus: () => void }>();
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
    // Staff who only punch attendance (isSalesperson off in kashi) don't sell,
    // and the till's own leftovers aren't offered once kashi manages the staff.
    salespersons.value = ((await ListSalespersons()) as Salesperson[])
      .filter((s: any) => s.isSalesperson !== false && s.isCurrent !== false)
      .sort((a, b) => a.fullName.localeCompare(b.fullName, undefined, { sensitivity: "base" }));
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
  // Filter by object identity, not barcode/code - a locked returned line and
  // an unlocked replacement line can share the same barcode in exchange
  // mode, and removing "the" line by barcode would take out both.
  items.value = items.value.filter((i) => i !== item);
}

function addItem(item: LineItem) {
  // A locked line (e.g. a returned line in exchange mode) is never a merge
  // target - scanning the same product as a replacement must add a
  // separate, editable line rather than incrementing the locked line's qty
  // (which would silently net a -1 return back toward 0).
  // Every unknown item is its own line: it needs its own pictures.
  const existing = isUnknownItem(item)
    ? undefined
    : items.value.find(
    (i) =>
      !i.locked &&
      ((item.barcode && i.barcode === item.barcode) || (!item.barcode && i.code === item.code)),
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
      originalPrice: item.originalPrice ?? price,
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
      locked: true,
      originalPrice: Number(item.originalPrice ?? item.price),
    })),
    ...replacements.map((item) => ({
      ...item,
      price: Number(item.price),
      qty: Number(item.qty || 1),
      unitPrices: Array(Math.max(1, Math.trunc(Number(item.qty || 1)))).fill(Number(item.price)),
      unitDiscounts: Array(Math.max(1, Math.trunc(Number(item.qty || 1)))).fill(
        Number(item.discount || 0),
      ),
      locked: false,
      originalPrice: Number(item.originalPrice ?? item.price),
    })),
  ];
}

function clearItems() {
  items.value = [];
  detailsDialog.value = false;
  detailItem.value = null;
}

// The "Unknown item" placeholder: its price is whatever was typed when it was
// added, one per unit, and is never edited afterwards (remove it and add it
// again to change it).
const UNKNOWN_BARCODE = "UNKNOWN";
function isUnknownItem(item: LineItem): boolean {
  return item.barcode === UNKNOWN_BARCODE;
}

function codeAllowsPriceEdit(item: LineItem): boolean {
  if (isUnknownItem(item)) return false;
  if (!props.priceEditRestricted) return true;
  return (props.priceEditCodes ?? []).includes((item.code ?? "").trim().toLowerCase());
}

function canEditPrice(item: LineItem): boolean {
  return (
    customPricesEnabled.value &&
    codeAllowsPriceEdit(item) &&
    !readonly.value &&
    !item.locked &&
    item.qty > 0
  );
}

function canEditDiscount(item: LineItem): boolean {
  // An unknown item's price is fixed when it is added, but its discount
  // follows the branch's custom-discount setting like any other line.
  return (
    customDiscountsEnabled.value &&
    (isUnknownItem(item) || codeAllowsPriceEdit(item)) &&
    !readonly.value &&
    !item.locked &&
    item.qty > 0
  );
}

function canEditUnitPrice(item: LineItem): boolean {
  return (
    item.qty > 0 &&
    (perUnitPricesEnabled.value || customPricesEnabled.value) &&
    codeAllowsPriceEdit(item) &&
    !readonly.value &&
    !item.locked
  );
}

function canEditUnitDiscount(item: LineItem): boolean {
  return (
    item.qty > 0 &&
    (perUnitPricesEnabled.value || customDiscountsEnabled.value) &&
    codeAllowsPriceEdit(item) &&
    !readonly.value &&
    !item.locked
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

function setFieldRef(
  index: number,
  column: FieldColumn,
  element: Element | ComponentPublicInstance | null,
) {
  const key = `${index}:${column}`;
  if (element && "focus" in element && typeof element.focus === "function") {
    fieldRefs.set(key, element as { focus: () => void });
  } else {
    fieldRefs.delete(key);
  }
}

// Which of qty/discount/price are editable (and thus keyboard-navigable) on
// a given line, in on-screen left-to-right order.
function columnsForItem(item: LineItem): FieldColumn[] {
  const columns: FieldColumn[] = [];
  if (!readonly.value && !item.locked && item.qty >= 0) columns.push("qty");
  if (canEditDiscount(item)) {
    columns.push("discount");
  }
  if (canEditPrice(item)) {
    columns.push("price");
  }
  return columns;
}

function setDraftDiscount(item: LineItem, value: unknown) {
  const numeric = Number(value);
  item.discount = Number.isFinite(numeric) ? Math.min(100, Math.max(0, Math.trunc(numeric))) : 0;
  if (priceChangeMode.value === "discount") {
    // Price and discount are the same lever - the base price never moves,
    // only the discount taken off it does.
    item.price = Math.round((item.originalPrice ?? item.price) * 100) / 100;
  }
  if (item.qty > 0) {
    item.unitPrices = Array(Math.trunc(item.qty)).fill(item.price);
    item.unitDiscounts = Array(Math.trunc(item.qty)).fill(item.discount);
  }
  // A manual edit always wins over a discount-list default, so the star
  // (which means "this came from a list") no longer applies.
  item.discountListOverridden = false;
  if (priceChangeMode.value === "discount") item.priceListOverridden = false;
}

function setDraftPrice(item: LineItem, value: unknown) {
  const numeric = Number(value);
  const clamped = Number.isFinite(numeric) ? Math.min(999999.99, Math.max(0.01, numeric)) : 0.01;
  if (priceChangeMode.value === "discount") {
    // The typed value is the price the cashier wants to charge; back it out
    // into a discount off the line's original price rather than replacing
    // the base price itself.
    const base = item.originalPrice && item.originalPrice > 0 ? item.originalPrice : item.price;
    const discount = base > 0 ? Math.min(100, Math.max(0, Math.round((1 - clamped / base) * 100))) : 0;
    item.price = Math.round(base * 100) / 100;
    item.discount = discount;
    item.discountListOverridden = false;
  } else {
    item.price = Math.round(clamped * 100) / 100;
  }
  if (item.qty > 0) {
    item.unitPrices = Array(Math.trunc(item.qty)).fill(item.price);
    item.unitDiscounts = Array(Math.trunc(item.qty)).fill(item.discount);
  }
  item.priceListOverridden = false;
}

function setUnitPrice(item: LineItem, index: number, value: unknown) {
  const numeric = Number(value);
  const clamped = Number.isFinite(numeric) ? Math.min(999999.99, Math.max(0.01, numeric)) : 0.01;
  ensureUnitPrices(item);
  if (priceChangeMode.value === "discount") {
    const base = item.originalPrice && item.originalPrice > 0 ? item.originalPrice : item.price;
    const discount = base > 0 ? Math.min(100, Math.max(0, Math.round((1 - clamped / base) * 100))) : 0;
    if (item.unitPrices) item.unitPrices[index] = Math.round(base * 100) / 100;
    if (item.unitDiscounts) item.unitDiscounts[index] = discount;
  } else {
    if (item.unitPrices) item.unitPrices[index] = Math.round(clamped * 100) / 100;
  }
}

function setUnitDiscount(item: LineItem, index: number, value: unknown) {
  const numeric = Number(value);
  const discount = Number.isFinite(numeric) ? Math.min(100, Math.max(0, Math.trunc(numeric))) : 0;
  ensureUnitPrices(item);
  if (item.unitDiscounts) item.unitDiscounts[index] = discount;
  if (priceChangeMode.value === "discount") {
    const base = item.originalPrice && item.originalPrice > 0 ? item.originalPrice : item.price;
    if (item.unitPrices) item.unitPrices[index] = Math.round(base * 100) / 100;
  }
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

async function focusField(index: number, column: FieldColumn) {
  invoiceScroll.value?.children[index]?.scrollIntoView({ block: "nearest" });
  await nextTick();
  requestAnimationFrame(() => fieldRefs.get(`${index}:${column}`)?.focus());
}

function moveHorizontal(index: number, column: FieldColumn, direction: -1 | 1) {
  const item = displayItems.value[index];
  if (!item) return;
  const columns = columnsForItem(item);
  const targetIndex = columns.indexOf(column) + direction;
  const target = columns[targetIndex];
  if (target === undefined) return;
  focusField(index, target);
}

// Walks rows in `direction` until one has `column` editable - a locked or
// zero-qty line simply has no discount/price field to land on.
function moveVertical(index: number, column: FieldColumn, direction: -1 | 1) {
  let target = index + direction;
  while (target >= 0 && target < displayItems.value.length) {
    const item = displayItems.value[target];
    if (item && columnsForItem(item).includes(column)) {
      focusField(target, column);
      return;
    }
    target += direction;
  }
}

async function moveFromDiscount(item: LineItem, index: number, direction: -1 | 1) {
  await persistDiscount(item);
  moveVertical(index, "discount", direction);
}

async function moveFromDiscountH(item: LineItem, index: number, direction: -1 | 1) {
  await persistDiscount(item);
  moveHorizontal(index, "discount", direction);
}

// --- Item details dialog: keyboard mode for the per-unit discount/price grid.
// Same shape as the main list's navigation, but rows are unit indexes on the
// one open line rather than invoice lines.
type UnitColumn = "discount" | "price";
const unitFieldRefs = new Map<string, { focus: () => void }>();

function setUnitFieldRef(
  unitIndex: number,
  column: UnitColumn,
  element: Element | ComponentPublicInstance | null,
) {
  const key = `${unitIndex}:${column}`;
  if (element && "focus" in element && typeof element.focus === "function") {
    unitFieldRefs.set(key, element as { focus: () => void });
  } else {
    unitFieldRefs.delete(key);
  }
}

function unitColumns(): UnitColumn[] {
  const item = detailItem.value;
  if (!item) return [];
  const columns: UnitColumn[] = [];
  if (canEditUnitDiscount(item)) columns.push("discount");
  if (canEditUnitPrice(item)) columns.push("price");
  return columns;
}

function moveUnit(unitIndex: number, column: UnitColumn, direction: -1 | 1) {
  const item = detailItem.value;
  if (!item) return;
  const target = unitIndex + direction;
  if (target < 0 || target >= detailUnitCount(item)) return;
  requestAnimationFrame(() => unitFieldRefs.get(`${target}:${column}`)?.focus());
}

function moveUnitH(unitIndex: number, column: UnitColumn, direction: -1 | 1) {
  const columns = unitColumns();
  const target = columns[columns.indexOf(column) + direction];
  if (target === undefined) return;
  requestAnimationFrame(() => unitFieldRefs.get(`${unitIndex}:${target}`)?.focus());
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

.invoice-price-value {
  align-items: center;
  display: flex;
  gap: 4px;
  justify-content: flex-end;
}

.invoice-price-values {
  align-items: flex-end;
  display: flex;
  flex-direction: column;
}

.invoice-price-editor:hover,
.invoice-price-editor:focus-visible {
  background: rgba(var(--v-theme-primary), 0.09);
  outline: none;
}

.invoice-price-editor--keyboard:focus {
  background: rgba(var(--v-theme-primary), 0.14);
  outline: 2px solid rgba(var(--v-theme-primary), 0.5);
  outline-offset: 1px;
}
</style>
