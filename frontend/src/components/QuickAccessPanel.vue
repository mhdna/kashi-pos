<template>
  <section
    class="quick-access"
    :class="{ 'quick-access--open': open }"
    :style="{ '--qa-font-scale': quickAccessFontScale / 100 }"
  >
    <div class="quick-access__toolbar">
      <div
        class="quick-access__toggle ps-2"
        @click="open = !open"
      >
        <k-icon :icon="open ? 'mdi-arrow-down' : 'mdi-button-pointer'" size="18" />
      </div>
      <ProductSearchDialog
        v-if="open && !managedByKashi"
        activator-text=""
        activator-icon="mdi-plus"
        @select="addPinnedItem"
      />
    </div>
    <div v-if="open" class="quick-access__body" :style="{ height: `${bodyHeight}px` }">
      <div v-if="!items.length" class="text-caption text-medium-emphasis text-center pa-2">
        {{ managedByKashi ? "No Quick Access cards set in kashi for this branch yet." : "Add frequently used items here." }}
      </div>
      <div ref="itemsEl" class="quick-access__items" :class="{ 'quick-access__items--dragging': dragIndex !== null }">
        <k-card
          v-for="(item, index) in items"
          :key="item.barcode || item.code"
          class="quick-access__item pa-2"
          :class="{
            'quick-access__item--dragging': dragIndex === index,
            'quick-access__item--over': dragOverIndex === index && dragIndex !== index,
            'quick-access__item--active': activeCode === item.code,
          }"
          :data-index="index"
          elevation="1"
          rounded="lg"
          :aria-label="item.name"
          :style="quickAccessCardStyle(item)"
          @pointerdown="onPointerDown(index, $event)"
          @click="onCardClick(item, $event)"
        >
          <!-- The invisible sizer always holds every line, so hiding the code or
               name from Settings never makes the cards shorter. -->
          <div class="quick-access__lines">
            <div class="quick-access__sizer" aria-hidden="true">
              <div class="quick-access__code text-caption">&nbsp;</div>
              <div class="quick-access__name text-caption font-weight-medium">&nbsp;</div>
              <div class="quick-access__price text-caption">&nbsp;</div>
            </div>
            <div class="quick-access__content">
              <div v-if="quickAccessShowCode" class="quick-access__code text-caption text-truncate">{{ item.code }}</div>
              <div v-if="quickAccessShowName" class="quick-access__name text-caption font-weight-medium text-truncate">{{ item.name }}</div>
              <div class="quick-access__price text-caption">
                <template v-if="item.priced !== false">${{ Number(item.price).toFixed(2) }}</template>
                <template v-else>No price</template>
              </div>
            </div>
          </div>
          <k-btn
            v-if="quickAccessShowRemove && !managedByKashi"
            icon
            size="x-small"
            variant="text"
            class="quick-access__remove"
            title="Remove item"
            @click.stop.prevent="removePinnedItem(item.code)"
          >
            <k-icon icon="mdi-close" size="14" />
          </k-btn>
        </k-card>
      </div>
    </div>
  </section>

  <k-dialog v-model="removeDialog" max-width="360">
    <k-card>
      <k-card-title>Remove quick-access item?</k-card-title>
      <k-card-text
        >Remove <strong>{{ removalName }}</strong> from quick access?</k-card-text
      >
      <k-card-actions>
        <k-spacer />
        <k-btn variant="text" @click="removeDialog = false">Cancel</k-btn>
        <k-btn color="error" variant="tonal" @click="confirmRemoval">Remove</k-btn>
      </k-card-actions>
    </k-card>
  </k-dialog>
</template>

<script lang="ts" setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import {
  quickAccessRows,
  quickAccessShowCode,
  quickAccessShowName,
  quickAccessShowRemove,
  quickAccessCardStyle,
  quickAccessFontScale,
} from "@/lib/display";
import ProductSearchDialog from "@/components/ProductSearchDialog.vue";
import {
  AddQuickAccessProduct,
  GetProductByCode,
  GetQuickAccessSource,
  ListQuickAccessProducts,
  RemoveQuickAccessProduct,
  ReorderQuickAccessProducts,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

type QuickAccessItem = {
  id?: number;
  name: string;
  code: string;
  category: string;
  price: number;
  /** false when no active price list prices it (shown as "No price"). */
  priced?: boolean;
  discount: number;
  qty: number;
  barcode?: string;
  /** #rrggbb set by an admin in Settings, or "" for the default card colour. */
  quickAccessColor?: string;
  /** #rrggbb text colour override, or "" to pick black/white automatically. */
  quickAccessTextColor?: string;
};

const emit = defineEmits<{ add: [item: QuickAccessItem] }>();
const STORAGE_KEY = "kashi-pos.quick-access";
const items = ref<QuickAccessItem[]>([]);
const open = ref(true);
const removeDialog = ref(false);
const removalCode = ref("");
const removalName = ref("");
const dragIndex = ref<number | null>(null);
const dragOverIndex = ref<number | null>(null);
const activeCode = ref<string | null>(null);
let activeTimer: ReturnType<typeof setTimeout> | undefined;

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

// Prices and products pushed from kashi update the cards in place, instead of
// waiting for the panel to be reopened. Skipped mid-drag so a reload can't
// yank a card out from under the cashier's finger.
let stopSyncListeners: (() => void)[] = [];
function reloadAfterSync() {
  if (dragIndex.value === null) void loadQuickAccess();
}
onMounted(() => {
  void loadQuickAccess();
  stopSyncListeners = ["products", "price_lists", "discount_lists", "quick_access"].map((entity) =>
    EventsOn(`sync_applied:${entity}`, reloadAfterSync),
  );
});
onBeforeUnmount(() => stopSyncListeners.forEach((stop) => stop()));

// The body is sized to show exactly `quickAccessRows` rows of cards (set by an
// admin in Settings); anything beyond that scrolls. Card height depends on the
// font size, so measure a real card instead of hard-coding it.
const ROW_GAP = 4; // matches .quick-access__items gap
const BODY_CHROME = 6; // .quick-access__items margin-top + .quick-access__body padding-bottom
// A few spare pixels, so a card a fraction of a pixel taller than measured
// doesn't bring up a scrollbar when every row fits.
const BODY_SLACK = 4;
const itemsEl = ref<HTMLElement | null>(null);
const rowHeight = ref(58);
const bodyHeight = computed(() => {
  const rows = Math.max(quickAccessRows.value, 1);
  return rows * rowHeight.value + (rows - 1) * ROW_GAP + BODY_CHROME + BODY_SLACK;
});

function measureRowHeight() {
  const card = itemsEl.value?.querySelector<HTMLElement>(".quick-access__item");
  // Not offsetHeight: it rounds, and a card of 58.4px measured as 58 left the
  // body short of its last row.
  const height = card?.getBoundingClientRect().height;
  if (height) rowHeight.value = Math.ceil(height);
}
const rowObserver = new ResizeObserver(measureRowHeight);
watch(
  [itemsEl, () => items.value.length],
  async () => {
    await nextTick();
    rowObserver.disconnect();
    const card = itemsEl.value?.querySelector<HTMLElement>(".quick-access__item");
    if (card) rowObserver.observe(card);
    measureRowHeight();
  },
  { immediate: true },
);
onBeforeUnmount(() => rowObserver.disconnect());

// In 'kashi' mode the cards are the branch's list from kashi: read-only here
// (no add, remove or drag). See kashi-pos's sync_quick_access.go.
const managedByKashi = ref(false);

async function loadQuickAccess() {
  const legacyItems = [...items.value];
  try {
    managedByKashi.value = (await GetQuickAccessSource()).mode === "kashi";
    let saved = ((await ListQuickAccessProducts()) as QuickAccessItem[]) ?? [];
    if (!saved.length && legacyItems.length && !managedByKashi.value) {
      for (const item of legacyItems) {
        try {
          const product = item.id ? item : ((await GetProductByCode(item.code)) as QuickAccessItem);
          if (product.id) await AddQuickAccessProduct(product.id);
        } catch {}
      }
      saved = ((await ListQuickAccessProducts()) as QuickAccessItem[]) ?? [];
    }
    items.value = saved.map((item) => ({ ...item, qty: 1 }));
  } catch {
    // Older backends still use the local copy until they are upgraded.
  }
}

async function addPinnedItem(item: QuickAccessItem) {
  if (items.value.some((existing) => existing.id === item.id || existing.code === item.code)) return;
  if (item.id) await AddQuickAccessProduct(item.id);
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
async function confirmRemoval() {
  const item = items.value.find((entry) => entry.code === removalCode.value);
  if (item?.id) await RemoveQuickAccessProduct(item.id);
  items.value = items.value.filter((entry) => entry.code !== removalCode.value);
  removeDialog.value = false;
  removalCode.value = "";
  removalName.value = "";
}

// HTML5 drag-and-drop never fires from touch input in the webview, so reorder
// with Pointer Events: press-and-hold a card briefly to pick it up, then drag.
// A plain tap still adds to the invoice, and a moving finger still scrolls the
// panel because we only take over once the finger has been held still.
const HOLD_MS = 250;
const MOVE_CANCEL = 10; // px of travel before the hold completes => it's a scroll/tap

let dragPointerId: number | null = null;
let holdTimer: ReturnType<typeof setTimeout> | undefined;
let startX = 0;
let startY = 0;
let dragging = false;
let suppressClick = false;
let sourceEl: HTMLElement | null = null;

function onPointerDown(index: number, event: PointerEvent) {
  if (managedByKashi.value) return; // kashi's order; taps still go to onCardClick
  if ((event.target as HTMLElement).closest(".quick-access__remove")) return;
  if (event.pointerType === "mouse" && event.button !== 0) return;

  suppressClick = false;
  dragPointerId = event.pointerId;
  startX = event.clientX;
  startY = event.clientY;
  dragging = false;
  sourceEl = event.currentTarget as HTMLElement;

  clearTimeout(holdTimer);
  holdTimer = setTimeout(() => beginDrag(index), HOLD_MS);

  window.addEventListener("pointermove", onPointerMove, { passive: false });
  window.addEventListener("pointerup", onPointerUp);
  window.addEventListener("pointercancel", onPointerUp);
}

function beginDrag(index: number) {
  dragging = true;
  suppressClick = true;
  dragIndex.value = index;
  dragOverIndex.value = index;
  try {
    if (dragPointerId !== null) sourceEl?.setPointerCapture(dragPointerId);
  } catch {}
  if (typeof navigator.vibrate === "function") navigator.vibrate(10);
}

function onPointerMove(event: PointerEvent) {
  if (event.pointerId !== dragPointerId) return;

  if (!dragging) {
    if (
      Math.abs(event.clientX - startX) > MOVE_CANCEL ||
      Math.abs(event.clientY - startY) > MOVE_CANCEL
    ) {
      clearTimeout(holdTimer);
      teardown();
    }
    return;
  }

  event.preventDefault();
  const under = document.elementFromPoint(event.clientX, event.clientY) as HTMLElement | null;
  const card = under?.closest<HTMLElement>(".quick-access__item[data-index]");
  if (card?.dataset.index !== undefined) dragOverIndex.value = Number(card.dataset.index);
}

function onPointerUp(event: PointerEvent) {
  if (dragPointerId !== null && event.pointerId !== dragPointerId) return;
  clearTimeout(holdTimer);
  if (dragging) void commitReorder(dragIndex.value, dragOverIndex.value);
  teardown();
}

function teardown() {
  window.removeEventListener("pointermove", onPointerMove);
  window.removeEventListener("pointerup", onPointerUp);
  window.removeEventListener("pointercancel", onPointerUp);
  try {
    if (sourceEl && dragPointerId !== null) sourceEl.releasePointerCapture(dragPointerId);
  } catch {}
  dragPointerId = null;
  sourceEl = null;
  dragging = false;
  dragIndex.value = null;
  dragOverIndex.value = null;
}

async function commitReorder(from: number | null, to: number | null) {
  if (from === null || to === null || from === to) return;
  const reordered = [...items.value];
  const moved = reordered.splice(from, 1)[0];
  if (!moved) return;
  reordered.splice(to, 0, moved);
  items.value = reordered;
  try {
    const ids = reordered.filter((item) => item.id).map((item) => item.id as number);
    if (ids.length) await ReorderQuickAccessProducts(ids);
  } catch {}
}

function onCardClick(item: QuickAccessItem, event: MouseEvent) {
  if (suppressClick) {
    suppressClick = false;
    event.preventDefault();
    event.stopPropagation();
    return;
  }
  activeCode.value = item.code;
  clearTimeout(activeTimer);
  activeTimer = setTimeout(() => {
    activeCode.value = null;
  }, 100);
  void addToInvoice(item);
}
</script>

<style scoped>
.quick-access {
  border: 1px solid rgba(var(--k-border-color), 0.12);
  border-radius: 12px;
  display: flex;
  flex: 0 0 42px;
  flex-direction: column;
  margin: 0 8px 8px 8px;
  overflow: hidden;
  transition: none !important;
}
.quick-access--open {
  flex: 0 1 auto;
  min-height: 0;
}
.quick-access:not(.quick-access--open) {
  align-self: flex-start;
  border: 0;
  margin-left: 8px;
  width: auto;
}
.quick-access__toolbar {
  align-items: center;
  display: flex;
  justify-content: space-between;
  min-height: 24px;
}
.quick-access__toggle {
  align-items: center;
  cursor: pointer;
  display: flex;
  min-height: 24px;
  overflow: hidden;
  transition: none !important;
  width: 40px;
}
.quick-access__toolbar :deep(.k-btn) {
  height: 28px !important;
  min-width: 28px;
  padding: 0 4px;
}
.quick-access:not(.quick-access--open) .quick-access__toggle {
  width: 128px;
}
.quick-access__body {
  min-height: 0;
  overflow-y: auto;
  padding: 0 8px 4px;
}
.quick-access__items {
  display: grid;
  gap: 4px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin-top: 2px;
}
.quick-access__item {
  cursor: grab;
  min-width: 0;
  min-height: 50px;
  padding: 6px !important;
  position: relative;
  transition: opacity 0.08s, outline-color 0.08s;
}
.quick-access__items--dragging .quick-access__item {
  touch-action: none;
  user-select: none;
}
.quick-access__item--dragging {
  opacity: 0.4;
}
.quick-access__item--over,
.quick-access__item--active {
  outline: 2px solid rgb(var(--k-theme-primary));
  outline-offset: -2px;
}
.quick-access__lines {
  display: grid;
  min-width: 0;
}
.quick-access__sizer,
.quick-access__content {
  grid-area: 1 / 1;
  min-width: 0;
}
.quick-access__sizer {
  visibility: hidden;
}
.quick-access__content {
  align-self: center;
}
.quick-access__content .quick-access__price {
  opacity: var(--k-medium-emphasis-opacity);
}
/* --qa-font-scale is the admin's "Quick access text size" from Settings. The
   sizer lines share these classes, so the card (and row height) grow with it. */
.quick-access__name {
  font-size: calc(var(--kashi-font-size-base) * var(--qa-font-scale, 1)) !important;
}
.quick-access__price {
  font-size: calc(var(--kashi-font-size-base) * var(--qa-font-scale, 1)) !important;
  line-height: 1.15;
}
.quick-access__code {
  font-size: calc(0.7rem * var(--qa-font-scale, 1)) !important;
  line-height: 1.15;
}
.quick-access__remove {
  background: rgb(var(--k-theme-background)) !important;
  height: 18px !important;
  position: absolute;
  right: 4px;
  top: 4px;
  width: 18px !important;
  z-index: 1;
}
</style>
