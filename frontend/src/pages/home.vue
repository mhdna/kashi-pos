<template>
  <div class="home-page d-flex h-100" style="width: 100%; overflow: hidden">
    <div class="home-left-pane d-flex flex-column" style="flex: 5 5 0; min-width: 0; min-height: 0">
      <div>
        <v-card-title
          class="d-flex justify-space-between"
          style="font-size: var(--kashi-font-size-2xl)"
        >
          <div class="d-flex flex-column">
            <div class="font-weight-bold">{{ pageModeTitle }}</div>
            <v-card-subtitle class="pa-0">{{ invoiceCode }}</v-card-subtitle>
          </div>
          <div class="d-flex flex-column align-end">
            <div class="d-flex">
              <v-btn icon size="small" variant="text" aria-label="Invoice note" @click="openNoteDialog">
                <v-icon icon="mdi-note-text-outline" size="20" />
              </v-btn>
              <v-dialog v-model="noteDialog" max-width="620">
                <v-card>
                  <v-card-title>Add a note</v-card-title>
                  <v-card-text>
                    <v-textarea
                      ref="noteInput"
                      v-model="noteDraft"
                      auto-grow
                      autofocus
                      hide-details
                      placeholder="Add a note"
                      rows="3"
                      variant="outlined"
                      @keydown.ctrl.enter="saveNote"
                    />
                    <InlineKeyboard v-model="noteDraft" :max-length="500" class="mt-3" />
                  </v-card-text>
                  <v-card-actions class="pa-4">
                    <v-spacer />
                    <v-btn variant="text" @click="noteDialog = false">Cancel</v-btn>
                    <v-btn color="primary" variant="elevated" @click="saveNote">Add</v-btn>
                  </v-card-actions>
                </v-card>
              </v-dialog>
              <v-btn
                v-if="pageMode !== 'sale'"
                color="red-darken-2"
                variant="tonal"
                prepend-icon="mdi-close"
                @click="cancelMode"
              >
                Cancel {{ pageMode }}
              </v-btn>
            </div>
          </div>
        </v-card-title>
      </div>
      <div class="d-flex align-center ms-4 me-0 mt-4">
        <v-text-field
          ref="barcodeInput"
          v-model="barcodeValue"
          class="flex-grow-1"
          density="compact"
          hide-details="auto"
          placeholder="Scan or Enter Barcode"
          :disabled="pageMode === 'settlement'"
          variant="outlined"
          @blur="onBlur"
          @keydown.enter="onBarcodeEnter"
        />
        <v-btn
          v-if="pageMode !== 'settlement'"
          variant="text"
          rounded
          aria-label="Search products"
          @click="productSearchDialog?.openDialog()"
        >
          <v-icon icon="mdi-magnify" size="20" />
        </v-btn>
        <ProductSearchDialog
          ref="productSearchDialog"
          v-if="pageMode !== 'settlement'"
          :show-activator="false"
          @select="onProductSelect"
        />
      </div>
      <InvoiceItems
        ref="invoiceItems"
        :rounding-mode="roundingMode"
        :readonly="pageMode === 'settlement'"
        :custom-discounts-enabled="customItemDiscountsEnabled"
        :custom-prices-enabled="customItemPricesEnabled"
        :per-unit-prices-enabled="perUnitItemPricesEnabled"
        style="flex: 1 1 0; min-height: 0"
        @discount-error="showDiscountError"
      />
      <QuickAccessPanel v-if="pageMode !== 'settlement'" @add="(item) => invoiceItems?.addItem(item)" />
      <!-- <InvoiceSummary
        :subtotal="invoiceItems?.subtotal ?? 0"
        :total-discount="invoiceItems?.totalDiscount ?? 0"
        :tax-rate="taxRate"
      /> -->
    </div>
    <div class="home-right-pane d-flex position-relative" style="flex: 5 5 0; min-width: 0">
      <div
        v-if="!hasItems"
        class="position-absolute d-flex align-center justify-center"
        style="
          top: 0;
          left: 0;
          right: 0;
          bottom: 0;
          background: rgba(0, 0, 0, 0.1);
          backdrop-filter: blur(4px);
          z-index: 10;
        "
      >
        <v-card class="pa-4 text-center" rounded="lg" width="240">
          <v-icon icon="mdi-barcode" size="32" class="mb-2" />
          <div class="text-body-2">Scan a barcode or add products to begin</div>
        </v-card>
      </div>
      <div class="mb-3 mt-1" style="flex: 3 3 0; min-width: 0">
        <Calculator
          ref="calculator"
          :total-cents="quote?.grandTotalCents ?? 0"
          :items-json="cartItemsJson"
          :client-id="actionBar?.clientId"
          :rounding-mode="roundingMode"
          :rounding-currency="roundingCurrency"
          :has-items="hasItems"
          :exchange-mode="pageMode === 'exchange'"
          :exchange-original-invoice-id="exchangeState?.originalInvoiceId"
          :settlement-invoice-id="settlementState?.invoiceId"
          @account="accountDialog = true"
          @coupon="calculator?.openCouponDialog()"
        />
      </div>
      <v-divider vertical class="ms-4 me-0" />
      <div style="flex: 2 2 0; min-width: 0">
        <ActionBar
          ref="actionBar"
          :total="invoiceTotal"
          :total-summary="calculator?.totalSummary ?? { usd: '$0.00', lbp: '' }"
          :can-checkout="canCheckout"
          :has-items="hasItems"
          :pulse-client="pulseClient"
          :mode="pageMode"
          @checkout="onCheckout"
          @hold="onHold"
          @void="onVoid"
          @account="accountDialog = true"
          @coupon="calculator?.openCouponDialog()"
        />
      </div>
    </div>
  </div>

  <AccountDialog v-model="accountDialog" @select="onAccountClientSelect" />

  <v-overlay v-model="printing" class="align-center justify-center" persistent>
    <v-card class="pa-8 text-center" width="300">
      <v-progress-circular indeterminate color="primary" size="64" class="mb-4" />
      <div class="text-h6">Printing receipt...</div>
    </v-card>
  </v-overlay>

  <v-snackbar
    v-model="snackbar.show"
    class="app-snackbar"
    timeout="3500"
    location="top"
    variant="flat"
  >
    {{ snackbar.text }}
  </v-snackbar>
</template>

<script lang="ts" setup>
import { computed, nextTick, onBeforeUnmount, onMounted, provide, reactive, ref, watch } from "vue";
import {
  GetProductByBarcode,
  GetProductByCode,
  HoldInvoice,
  VoidInvoice,
  CheckoutInvoice,
  CheckoutExchangeInvoice,
  SettleInvoicePayments,
  GetNextInvoiceCode,
  GetAppSettings,
  DeleteHoldInvoice,
  GetOpenShift,
  OpenShift,
  WriteToScreen,
  QuoteInvoiceTotal,
  QuoteExchangeTotal,
} from "../../wailsjs/go/main/App";
import ActionBar from "@/components/ActionBar.vue";
import AccountDialog from "@/components/AccountDialog.vue";
import Calculator from "@/components/Calculator.vue";
import InvoiceItems from "@/components/InvoiceItems.vue";
import InlineKeyboard from "@/components/InlineKeyboard.vue";
// import InvoiceSummary from "@/components/InvoiceSummary.vue";
import ProductSearchDialog from "@/components/ProductSearchDialog.vue";
import QuickAccessPanel from "@/components/QuickAccessPanel.vue";
import type { ExchangeSelection } from "@/lib/exchange";
import { notifyIncomeChanged } from "@/lib/incomeEvents";
import { normalizeRoundingMode, type RoundingMode } from "@/lib/money";
import type { Client } from "@/lib/client";
import { useSalesperson } from "@/composables/useSalesperson";
import {
  pendingExchangeSelection,
  pendingHoldInvoice,
  pendingSettlementInvoice,
} from "@/composables/useNavActions";

const INVOICE_DRAFT_KEY = "kashi-pos.invoice-draft";
type InvoiceDraft = {
  items: any[];
  client: Client;
  notes?: string;
  // Set only while an exchange is in progress, so an unfinished exchange
  // (returned lines locked in, replacements maybe scanned) survives an app
  // restart or logout/login instead of silently reverting to a normal sale.
  exchangeOriginalInvoiceId?: number;
};
const { selectedSalesperson, pulseSalesperson } = useSalesperson();
let pageActive = true;

const taxRate = ref(0);
const roundingMode = ref<RoundingMode>("half_up");
const roundingCurrency = ref("");
const receiptEnabled = ref(false);
const receiptPrinter = ref("");
const customItemDiscountsEnabled = ref(false);
const customItemPricesEnabled = ref(false);
const perUnitItemPricesEnabled = ref(false);
const pulseClient = ref(false);
let pulseClientTimer: ReturnType<typeof setTimeout> | undefined;
const branchName = ref("Store");
const screenPort = ref("");
const invoiceCode = ref("Loading invoice code…");
const invoiceNotes = ref("");
const noteDialog = ref(false);
const noteDraft = ref("");
const exchangeState = ref<{ originalInvoiceId: number } | null>(null);
const settlementState = ref<{
  invoiceId: number;
  invoiceCode: string;
  grandTotalCents: number;
} | null>(null);
// The single source of truth for which of the three home-screen modes is
// active - everything else (locking items/client/salesperson, which
// controls are visible, the checkout button's label/color) branches off
// this instead of checking exchangeState/settlementState directly.
const pageMode = computed<"sale" | "exchange" | "settlement">(() =>
  settlementState.value ? "settlement" : exchangeState.value ? "exchange" : "sale",
);
const pageModeTitle = computed(() =>
  pageMode.value === "settlement"
    ? "Change Settlement"
    : pageMode.value === "exchange"
      ? "Exchange"
      : "Invoice",
);
const invoiceItems = ref<InstanceType<typeof InvoiceItems> | null>(null);
const calculator = ref<InstanceType<typeof Calculator> | null>(null);
const actionBar = ref<InstanceType<typeof ActionBar> | null>(null);
const productSearchDialog = ref<InstanceType<typeof ProductSearchDialog> | null>(null);
const accountDialog = ref(false);

// The backend quote is the single source of truth for the cart total - see
// refreshQuote()/getFullItemsJSON() below. invoiceTotal/cartItemsJson are
// kept in sync by the same debounced watcher so the calculator's total and
// payment-status checks are always priced from the exact same cart JSON.
interface InvoiceQuote {
  grandTotalCents: number;
}
const quote = ref<InvoiceQuote | null>(null);
const cartItemsJson = ref("");
const invoiceTotal = computed(() => (quote.value?.grandTotalCents ?? 0) / 100);
const hasItems = computed(() => (invoiceItems.value?.items?.length ?? 0) > 0);
const clientSelected = computed(() => Boolean(actionBar.value?.clientSelected));
const salespersonSelected = computed(() => Boolean(selectedSalesperson.value));
const canCheckout = computed(() => {
  if (!calculator.value?.checkoutReady) return false;
  // Settlement mode's client/salesperson/items are pre-set and frozen from
  // the loaded invoice - only the payment balance gates checkout.
  if (pageMode.value === "settlement") return true;
  return (
    (pageMode.value === "exchange" ? hasItems.value : invoiceTotal.value > 0) &&
    clientSelected.value &&
    salespersonSelected.value
  );
});

const barcodeValue = ref("");
const barcodeInput = ref<InstanceType<typeof import("vuetify/components").VTextField> | null>(null);
const noteInput = ref<InstanceType<typeof import("vuetify/components").VTextarea> | null>(null);
const dialogOpen = ref(false);
let overlayObserver: MutationObserver | undefined;
let overlayFocusTimer: ReturnType<typeof setTimeout> | undefined;
let restoringDraft = true;

provide("dialogOpen", dialogOpen);

watch(hasItems, (val) => {
  if (!val) calculator.value?.clearAll();
});

watch(dialogOpen, (v) => {
  if (!v) focusBarcodeWhenIdle();
});

watch(noteDialog, (open) => {
  if (!open) return;
  noteDraft.value = invoiceNotes.value;
  void nextTick(() => noteInput.value?.focus());
});

watch(pendingHoldInvoice, () => consumePendingHold());
watch(pendingExchangeSelection, () => void consumePendingExchange());
watch(pendingSettlementInvoice, () => void consumePendingSettlement());

watch(
  [() => invoiceItems.value?.items, () => actionBar.value?.client, invoiceNotes, exchangeState],
  ([items, client]) => {
    // Settlement mode's cart is a frozen snapshot of one specific invoice,
    // not a draft sale - it must never be persisted/restored as one.
    if (restoringDraft || !client || settlementState.value) return;
    if (!items?.length && client.id === 0 && !exchangeState.value) {
      localStorage.removeItem(INVOICE_DRAFT_KEY);
      return;
    }
    localStorage.setItem(
      INVOICE_DRAFT_KEY,
      JSON.stringify({
        items: items ?? [],
        client,
        notes: invoiceNotes.value,
        exchangeOriginalInvoiceId: exchangeState.value?.originalInvoiceId,
      } satisfies InvoiceDraft),
    );
  },
  { deep: true },
);

let quoteRequestToken = 0;
let quoteDebounceTimer: ReturnType<typeof setTimeout> | undefined;

async function refreshQuote() {
  const token = ++quoteRequestToken;
  const itemsJson = await getFullItemsJSON();
  if (token !== quoteRequestToken) return; // a newer request already superseded this one
  cartItemsJson.value = itemsJson;
  if (settlementState.value) {
    // Settlement mode never re-prices - the total is the invoice's frozen
    // GrandTotal, captured once when it was loaded. Re-running it through
    // QuoteInvoiceTotal could drift from that total if pricing/tax/rounding
    // settings changed since the original sale.
    quote.value = { grandTotalCents: settlementState.value.grandTotalCents };
    return;
  }
  if (itemsJson === "[]") {
    quote.value = null;
    return;
  }
  if (exchangeState.value) {
    // A cart with negative-qty returned lines errors out of
    // QuoteInvoiceTotal outright (it only ever accepts qty > 0), so exchange
    // mode prices through the dedicated exchange quote instead - the same
    // pricing checkoutReturnExchangeInvoice will actually apply.
    try {
      const result = await QuoteExchangeTotal(itemsJson, exchangeState.value.originalInvoiceId);
      if (token !== quoteRequestToken) return;
      quote.value = { grandTotalCents: Number(result.grandTotalCents) };
    } catch (e) {
      if (token !== quoteRequestToken) return;
      console.error("QuoteExchangeTotal failed:", e);
      quote.value = null;
    }
    return;
  }
  try {
    const result = await QuoteInvoiceTotal(itemsJson);
    if (token !== quoteRequestToken) return;
    quote.value = { grandTotalCents: Number(result.grandTotalCents) };
  } catch (e) {
    if (token !== quoteRequestToken) return;
    // Surfaced instead of silently showing $0 - if this fires, the console
    // has the actual reason (a real pricing error, or QuoteInvoiceTotal not
    // existing on window.go yet because the app needs a rebuild/restart).
    console.error("QuoteInvoiceTotal failed:", e);
    quote.value = null;
  }
}

watch(
  [
    () => invoiceItems.value?.items,
    roundingMode,
    roundingCurrency,
    taxRate,
    customItemPricesEnabled,
    perUnitItemPricesEnabled,
    customItemDiscountsEnabled,
  ],
  () => {
    clearTimeout(quoteDebounceTimer);
    quoteDebounceTimer = setTimeout(refreshQuote, 120);
  },
  { deep: true, immediate: true },
);

async function restoreDraft() {
  try {
    const draft = JSON.parse(
      localStorage.getItem(INVOICE_DRAFT_KEY) || "null",
    ) as InvoiceDraft | null;
    if (!draft) return;
    // Assigned directly (bypassing addItem's scan-one-unit-at-a-time merge,
    // which clamps qty to a minimum of 1) so a locked, negative-qty returned
    // line from an in-progress exchange restores exactly as it was, instead
    // of being corrupted into a positive qty-1 line.
    if (invoiceItems.value) {
      invoiceItems.value.items = (draft.items ?? []).map((item) => ({ ...item }));
    }
    if (draft.client?.id) actionBar.value?.setClient(draft.client);
    invoiceNotes.value = draft.notes ?? "";
    noteDraft.value = invoiceNotes.value;
    if (draft.exchangeOriginalInvoiceId) {
      exchangeState.value = { originalInvoiceId: draft.exchangeOriginalInvoiceId };
      await refreshNextInvoiceCode("exchange");
    }
  } catch {
    localStorage.removeItem(INVOICE_DRAFT_KEY);
  } finally {
    restoringDraft = false;
  }
}

function clearDraft() {
  localStorage.removeItem(INVOICE_DRAFT_KEY);
}

function clearInvoice() {
  invoiceItems.value?.clearItems();
  calculator.value?.clearAll();
  // Keep the salesperson selected for the next invoice. Restoring an on-hold
  // invoice still replaces this below with the salesperson saved on that hold.
  invoiceNotes.value = "";
  noteDraft.value = "";
  noteDialog.value = false;
  clearDraft();
  actionBar.value?.resetClient();
}

function saveNote() {
  invoiceNotes.value = noteDraft.value.trim();
  noteDialog.value = false;
  focusBarcode();
}

function openNoteDialog() {
  noteDraft.value = invoiceNotes.value;
  noteDialog.value = true;
}

function focusBarcode() {
  barcodeInput.value?.focus();
}

function hasBlockingUi() {
  return (
    dialogOpen.value ||
    Boolean(
      document.querySelector(
        ".v-overlay--active, .v-menu__content, [role='dialog'], [role='menu']",
      ),
    )
  );
}

function focusBarcodeWhenIdle(attempt = 0) {
  if (hasBlockingUi()) {
    if (attempt < 20) requestAnimationFrame(() => focusBarcodeWhenIdle(attempt + 1));
    return;
  }
  focusBarcode();
}

function scheduleBarcodeFocus() {
  clearTimeout(overlayFocusTimer);
  overlayFocusTimer = setTimeout(() => focusBarcodeWhenIdle(), 50);
}

function onBlur() {
  setTimeout(() => {
    if (hasBlockingUi()) return;
    const active = document.activeElement;
    if (
      active instanceof HTMLElement &&
      (active.matches("input, textarea, select, [contenteditable='true']") ||
        Boolean(active.closest("[role='dialog'], [role='menu'], .v-overlay__content")))
    ) {
      return;
    }
    focusBarcode();
  });
}

function onProductSelect(product: {
  name: string;
  code: string;
  category: string;
  price: number;
  discount: number;
  qty: number;
}) {
  invoiceItems.value?.addItem(product);
  focusBarcode();
}

function onAccountClientSelect(client: Client) {
  actionBar.value?.setClient(client);
  nextTick(() => calculator.value?.addOnAccount());
}

async function onBarcodeEnter() {
  const value = barcodeValue.value;
  if (!value) return;
  let match: any = null;
  try {
    const p = await GetProductByBarcode(value);
    match = { ...p, qty: 1 };
  } catch {
    try {
      const p = await GetProductByCode(value);
      match = { ...p, qty: 1 };
    } catch {}
  }
  if (match) invoiceItems.value?.addItem(match);
  barcodeValue.value = "";
  focusBarcode();
}

function showDiscountError(message: string) {
  snackbar.text = message || "Could not save discount.";
  snackbar.color = "red-darken-4";
  snackbar.show = true;
}

const printing = ref(false);
const snackbar = reactive({ show: false, text: "", color: "success" });

let screenQueue = Promise.resolve();
function queueScreenUpdate(message: string) {
  const port = screenPort.value.trim();
  if (!port) return;
  screenQueue = screenQueue
    .then(() => WriteToScreen(port, message))
    .catch((error) => console.warn("Customer screen update failed:", error));
}

function showCustomerTotal() {
  if (!hasItems.value) {
    queueScreenUpdate("");
    return;
  }
  queueScreenUpdate(invoiceTotal.value.toFixed(2));
}

watch([invoiceTotal, hasItems], showCustomerTotal);

async function getItemsJSON(): Promise<string> {
  const items = invoiceItems.value?.items ?? [];
  return JSON.stringify(items.map((i: any) => ({ code: i.code, barcode: i.barcode, qty: i.qty })));
}

async function getFullItemsJSON(): Promise<string> {
  const items = invoiceItems.value?.items ?? [];
  return JSON.stringify(
    items.map((i: any) => ({
      name: i.name,
      code: i.code,
      category: i.category,
      price: i.price,
      customPriceCents: customItemPricesEnabled.value
        ? Math.round(Number(i.price) * 100)
        : undefined,
      unitPricesCents:
        (perUnitItemPricesEnabled.value || customItemPricesEnabled.value) && i.qty > 0
          ? (i.unitPrices ?? Array(Math.trunc(i.qty)).fill(i.price)).map((price: number) =>
              Math.round(Number(price) * 100),
            )
          : undefined,
      unitDiscounts:
        (perUnitItemPricesEnabled.value || customItemDiscountsEnabled.value) && i.qty > 0
          ? (i.unitDiscounts ?? Array(Math.trunc(i.qty)).fill(i.discount)).map((discount: number) =>
              Math.trunc(Number(discount)),
            )
          : undefined,
      unitPriceCents: i.qty < 0 ? Math.round(Number(i.soldUnitPrice ?? i.price) * 100) : undefined,
      unitDiscount: i.qty < 0 ? Math.trunc(Number(i.soldUnitDiscount ?? i.discount)) : undefined,
      discount: i.discount,
      salespersonId: Number(i.salespersonId || 0),
      note: String(i.note || "").trim(),
      qty: i.qty,
    })),
  );
}

async function onHold() {
  if (invoiceItems.value && !(await invoiceItems.value.flushChanges())) return;
  const items = invoiceItems.value?.items ?? [];
  if (!items.length) return;
  await HoldInvoice(
    JSON.stringify({
      version: 1,
      items,
      client: actionBar.value?.client,
      salesperson: selectedSalesperson.value,
      calculator: calculator.value?.holdState(),
      notes: invoiceNotes.value,
    }),
  );
  clearInvoice();
}

async function onVoid() {
  const json = await getFullItemsJSON();
  if (json === "[]") return;
  await VoidInvoice(json, "");
  clearInvoice();
}

async function onSettlementCheckout() {
  if (!settlementState.value) return;
  printing.value = false;
  try {
    const paymentsJSON = calculator.value?.paymentsJSON ?? "[]";
    const result = await SettleInvoicePayments(settlementState.value.invoiceId, paymentsJSON);
    snackbar.text = `Settlement updated — ${result?.invoiceCode ?? settlementState.value.invoiceCode}`;
    snackbar.color = "success";
    snackbar.show = true;
    clearInvoice();
    settlementState.value = null;
    notifyIncomeChanged();
    await refreshNextInvoiceCode("sales");
  } catch (e: any) {
    snackbar.text = e?.toString() || "Could not update settlement";
    snackbar.color = "red-darken-4";
    snackbar.show = true;
  }
}

async function onCheckout() {
  if (pageMode.value === "settlement") {
    await onSettlementCheckout();
    return;
  }
  if (invoiceItems.value && !(await invoiceItems.value.flushChanges())) return;
  const json = await getFullItemsJSON();
  if (json === "[]") return;
  const isExchange = Boolean(exchangeState.value);
  const missingClient = !clientSelected.value;
  const missingSalesperson = !salespersonSelected.value;
  if (missingClient || missingSalesperson) {
    pulseClient.value = false;
    nextTick(() => {
      pulseClient.value = missingClient;
      clearTimeout(pulseClientTimer);
      pulseClientTimer = setTimeout(() => {
        pulseClient.value = false;
      }, 1500);
      if (missingSalesperson) pulseSalesperson();
    });
    snackbar.text = `Select ${[missingSalesperson && "a salesperson", missingClient && "a client"].filter(Boolean).join(" and ")} before checkout.`;
    snackbar.color = "warning";
    snackbar.show = true;
    return;
  }
  printing.value = receiptEnabled.value;
  try {
    const paymentsJSON = calculator.value?.paymentsJSON ?? "[]";
    const clientID = actionBar.value?.clientId ?? 0;
    const salesperson = selectedSalesperson.value;
    const result = exchangeState.value
      ? await CheckoutExchangeInvoice(
          json,
          paymentsJSON,
          exchangeState.value.originalInvoiceId,
          clientID,
          salesperson!.id,
          invoiceNotes.value,
        )
      : await CheckoutInvoice(
          json,
          paymentsJSON,
          clientID,
          branchName.value,
          branchName.value,
          "",
          salesperson!.fullName,
          receiptPrinter.value,
          invoiceNotes.value,
        );
    snackbar.text = result?.receiptError
      ? `Invoice created; receipt failed — ${result.receiptError}`
      : isExchange
        ? `Exchange completed — ${result?.invoiceCode ?? ""}`
        : receiptEnabled.value
          ? `Receipt printed — ${result?.invoiceCode ?? ""}`
          : `Invoice created — ${result?.invoiceCode ?? ""}`;
    snackbar.color = "success";
    snackbar.show = true;
    clearInvoice();
    exchangeState.value = null;
    notifyIncomeChanged();
    await nextTick();
    queueScreenUpdate("");
    await refreshNextInvoiceCode("sales");
  } catch (e: any) {
    snackbar.text = e?.toString() || "Checkout failed";
    snackbar.color = "red-darken-4";
    snackbar.show = true;
  }
  printing.value = false;
}

async function onExchange(payload: ExchangeSelection) {
  clearInvoice();
  invoiceItems.value?.addExchangeItems(
    payload.returned.map((item) => ({
      ...item,
      category: item.category ?? "",
      qty: Number(item.qty || 1),
    })),
    [],
  );
  actionBar.value?.setClient(payload.client);
  exchangeState.value = { originalInvoiceId: payload.originalInvoiceId };
  await refreshNextInvoiceCode("exchange");
  calculator.value?.clearAll();
  snackbar.text = "Exchange loaded. Scan or search replacement items, then check out.";
  snackbar.color = "info";
  snackbar.show = true;
}

async function cancelMode() {
  clearInvoice();
  calculator.value?.clearAll();
  exchangeState.value = null;
  settlementState.value = null;
  actionBar.value?.resetClient();
  await refreshNextInvoiceCode("sales");
}

async function refreshNextInvoiceCode(kind: "sales" | "exchange") {
  try {
    invoiceCode.value = await GetNextInvoiceCode(kind);
  } catch {
    invoiceCode.value = "Invoice code unavailable";
  }
}

function onRestoreHold(hold: any) {
  if (!invoiceItems.value) return;
  invoiceItems.value.clearItems();
  for (const item of hold.items ?? []) {
    invoiceItems.value.addItem({ ...item, qty: item.qty || 1 });
  }
  if (hold.client?.id) actionBar.value?.setClient(hold.client);
  else actionBar.value?.resetClient();
  selectedSalesperson.value = hold.salesperson ?? null;
  invoiceNotes.value = hold.notes ?? "";
  noteDraft.value = invoiceNotes.value;
  calculator.value?.restoreHoldState(hold.calculator);
  void DeleteHoldInvoice(hold.id);
}

function consumePendingHold() {
  const pending = pendingHoldInvoice.value;
  if (!pending) return;
  pendingHoldInvoice.value = null;
  onRestoreHold(pending);
}

async function consumePendingExchange() {
  const pending = pendingExchangeSelection.value;
  if (!pending) return;
  pendingExchangeSelection.value = null;
  await onExchange(pending);
}

async function consumePendingSettlement() {
  const pending = pendingSettlementInvoice.value;
  if (!pending || !invoiceItems.value) return;
  pendingSettlementInvoice.value = null;
  clearInvoice();
  // Assigned directly (bypassing addItem's scan-one-unit-at-a-time merge
  // logic) so each priced line from the invoice - including multiple
  // buckets of the same product at different custom prices - loads exactly
  // as it was, instead of being folded into one averaged line.
  invoiceItems.value.items = pending.items.map((item: any) => {
    const qty = Math.max(1, Math.trunc(Number(item.qty || 1)));
    return {
      name: item.name,
      code: item.code,
      barcode: item.barcode,
      category: item.category ?? "",
      color: item.color,
      size: item.size,
      price: Number(item.price),
      discount: Number(item.discount || 0),
      qty,
      salespersonId: Number(item.salespersonId || 0),
      note: item.note ?? "",
      priceListOverridden: Boolean(item.isPriceOverridden),
      priceListName: item.priceListName,
      discountListOverridden: Boolean(item.isDiscountOverridden),
      discountListName: item.discountListName,
      unitPrices: Array(qty).fill(Number(item.price)),
      unitDiscounts: Array(qty).fill(Number(item.discount || 0)),
    };
  });
  if (pending.client?.id) actionBar.value?.setClient(pending.client);
  selectedSalesperson.value = pending.salesperson ?? null;
  invoiceNotes.value = pending.notes ?? "";
  noteDraft.value = invoiceNotes.value;
  settlementState.value = {
    invoiceId: pending.id,
    invoiceCode: pending.invoiceCode,
    grandTotalCents: Number(pending.grandTotalCents || 0),
  };
  invoiceCode.value = pending.invoiceCode;
  await nextTick();
  calculator.value?.restoreHoldState({ entries: pending.entries ?? [] });
}

onMounted(async () => {
  await nextTick();
  if (!pageActive) return;
  overlayObserver = new MutationObserver(scheduleBarcodeFocus);
  overlayObserver.observe(document.body, {
    attributes: true,
    attributeFilter: ["class", "style"],
    childList: true,
    subtree: true,
  });
  await restoreDraft();
  consumePendingHold();
  await consumePendingExchange();
  await consumePendingSettlement();
  focusBarcode();
  try {
    const settings = await GetAppSettings();
    if (!pageActive) return;
    taxRate.value = Number(settings?.taxRate ?? 0);
    roundingMode.value = normalizeRoundingMode(settings?.roundingMode ?? "");
    roundingCurrency.value = settings?.roundingCurrency ?? "";
    receiptEnabled.value = Boolean(settings?.receiptEnabled);
    receiptPrinter.value = settings?.receiptPrinter ?? "";
    customItemDiscountsEnabled.value = Boolean(settings?.customItemDiscountsEnabled);
    customItemPricesEnabled.value = Boolean(settings?.customItemPricesEnabled);
    perUnitItemPricesEnabled.value = Boolean(settings?.perUnitItemPricesEnabled);
    screenPort.value = settings?.screenPort ?? "";
    branchName.value = settings?.branchName || "Store";
  } catch {}
  if (!pageActive) return;
  try {
    await GetOpenShift();
  } catch {
    if (!pageActive) return;
    try {
      await OpenShift();
    } catch {}
  }
  if (!pageActive) return;
  showCustomerTotal();
  // A restored exchange draft (or a pending hold/exchange/settlement
  // consumed above) already set the invoice code for its own mode - don't
  // clobber it back to a plain "sales" code.
  if (pageMode.value === "sale") await refreshNextInvoiceCode("sales");
});

onBeforeUnmount(() => {
  pageActive = false;
  clearTimeout(pulseClientTimer);
  clearTimeout(overlayFocusTimer);
  overlayObserver?.disconnect();
});
</script>

<style>
.home-right-pane {
  background-color: var(--kashi-app-background);
  border-left: 1px solid rgba(var(--v-border-color), 0.18);
}

.home-left-pane {
  background-color: var(--kashi-app-background);
}

.home-page .bg-surface-light {
  background-color: rgba(var(--v-theme-on-surface), 0.04) !important;
}

.app-snackbar .v-snackbar__wrapper {
  background: rgba(var(--v-theme-surface-variant), 0.72) !important;
  color: rgb(var(--v-theme-on-surface-variant)) !important;
  border: 1px solid rgba(var(--v-theme-on-surface-variant), 0.1);
  box-shadow: 0 4px 18px rgba(0, 0, 0, 0.12) !important;
}
</style>
