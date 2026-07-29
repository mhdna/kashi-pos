<template>
  <div class="d-flex h-100" style="width: 100%; overflow: hidden">
    <div class="d-flex flex-column" style="flex: 5 5 0; min-width: 0; min-height: 0">
      <div>
        <v-card-title class="d-flex justify-space-between">
          <div>{{ exchangeState ? "Exchange" : "Invoice" }}</div>
          <div class="d-flex">
            <v-menu v-model="noteMenu" :close-on-content-click="false" location="bottom end">
              <template #activator="{ props }">
                <v-btn
                  v-bind="props"
                  icon
                  size="small"
                  variant="text"
                  :color="invoiceNotes ? 'primary' : undefined"
                  class="me-1"
                >
                  <v-icon icon="mdi-note-text-outline" size="20" />
                </v-btn>
              </template>
              <v-card class="pa-3" min-width="280">
                <v-textarea
                  v-model="noteDraft"
                  auto-grow
                  density="compact"
                  hide-details
                  placeholder="Add a note"
                  rows="3"
                  variant="outlined"
                />
                <div class="d-flex justify-end mt-2">
                  <v-btn color="primary" variant="tonal" @click="saveNote">
                    <v-icon icon="mdi-plus" size="18" class="me-1" />
                    Add
                  </v-btn>
                </div>
              </v-card>
            </v-menu>
            <div class="me-2">
              <OnHoldMenu v-if="!exchangeState" @restore="onRestoreHold" />
            </div>
            <ReturnInvoiceMenu
              v-if="!exchangeState"
              :rounding-mode="roundingMode"
              @exchange="onExchange"
            />
            <v-btn
              v-else
              color="red-darken-2"
              variant="tonal"
              prepend-icon="mdi-close"
              @click="cancelExchange"
            >
              Cancel exchange
            </v-btn>
          </div>
        </v-card-title>
        <v-card-subtitle>{{ invoiceCode }}</v-card-subtitle>
      </div>
      <div class="d-flex align-center mx-4 mt-4">
        <v-text-field
          ref="barcodeInput"
          v-model="barcodeValue"
          class="flex-grow-1"
          density="compact"
          hide-details="auto"
          placeholder="Scan or Enter Barcode"
          :disabled="Boolean(exchangeState)"
          variant="outlined"
          @blur="onBlur"
          @keydown.enter="onBarcodeEnter"
        />
        <v-btn
          v-if="!exchangeState"
          icon
          size="small"
          variant="text"
          class="ms-1"
          @click="onBarcodeEnter"
        >
          <v-icon icon="mdi-magnify" size="20" />
        </v-btn>
        <ProductSearchDialog v-if="!exchangeState" @select="onProductSelect" />
      </div>
      <InvoiceItems
        ref="invoiceItems"
        :rounding-mode="roundingMode"
        :readonly="Boolean(exchangeState)"
        style="flex: 1 1 0; min-height: 0"
      />
      <QuickAccessPanel v-if="!exchangeState" @add="(item) => invoiceItems?.addItem(item)" />
      <InvoiceSummary
        :subtotal="invoiceItems?.subtotal ?? 0"
        :total-discount="invoiceItems?.totalDiscount ?? 0"
        :tax-rate="taxRate"
      />
    </div>
    <div class="d-flex position-relative" style="flex: 5 5 0; min-width: 0">
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
        <Calculator ref="calculator" :total="invoiceTotal" :rounding-mode="roundingMode" />
      </div>
      <div style="flex: 2 2 0; min-width: 0">
        <ActionBar
          ref="actionBar"
          :total="invoiceTotal"
          :can-checkout="canCheckout"
          :has-items="hasItems"
          :pulse-client="pulseClient"
          :exchange-mode="Boolean(exchangeState)"
          @hold="onHold"
          @void="onVoid"
          @checkout="onCheckout"
        />
      </div>
    </div>
  </div>

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
import { ref, provide, watch, onMounted, computed, reactive, nextTick } from "vue";
import {
  GetProductByBarcode,
  ListProducts,
  HoldInvoice,
  VoidInvoice,
  CheckoutInvoice,
  CheckoutExchangeInvoice,
  GetNextInvoiceCode,
  GetAppSettings,
  DeleteHoldInvoice,
  GetOpenShift,
  OpenShift,
  WriteToScreen,
} from "../../wailsjs/go/main/App";
import ActionBar from "@/components/ActionBar.vue";
import Calculator from "@/components/Calculator.vue";
import InvoiceItems from "@/components/InvoiceItems.vue";
import InvoiceSummary from "@/components/InvoiceSummary.vue";
import OnHoldMenu from "@/components/OnHoldMenu.vue";
import ProductSearchDialog from "@/components/ProductSearchDialog.vue";
import QuickAccessPanel from "@/components/QuickAccessPanel.vue";
import ReturnInvoiceMenu from "@/components/ReturnInvoiceMenu.vue";
import type { ExchangeSelection } from "@/lib/exchange";
import { roundCents } from "@/lib/money";
import type { Client } from "@/lib/client";
import { useSalesperson } from "@/composables/useSalesperson";

const INVOICE_DRAFT_KEY = "kashi-pos.invoice-draft";
type InvoiceDraft = { items: any[]; client: Client; notes?: string };
const { selectedSalesperson, pulseSalesperson } = useSalesperson();

const taxRate = ref(0);
const roundingMode = ref("half_up");
const receiptEnabled = ref(false);
const receiptPrinter = ref("");
const pulseClient = ref(false);
let pulseClientTimer: ReturnType<typeof setTimeout> | undefined;
const branchName = ref("Store");
const screenPort = ref("");
const invoiceCode = ref("Loading invoice code…");
const invoiceNotes = ref("");
const noteMenu = ref(false);
const noteDraft = ref("");
const exchangeState = ref<{ originalInvoiceId: number } | null>(null);
const invoiceItems = ref<InstanceType<typeof InvoiceItems> | null>(null);
const calculator = ref<InstanceType<typeof Calculator> | null>(null);
const actionBar = ref<InstanceType<typeof ActionBar> | null>(null);

const invoiceTotal = computed(() => {
  const discountedCents = invoiceItems.value?.discountedSubtotalCents ?? 0;
  const taxCents = roundCents((discountedCents / 100) * taxRate.value * 100, roundingMode.value);
  return (discountedCents + taxCents) / 100;
});
const hasItems = computed(() => (invoiceItems.value?.items?.length ?? 0) > 0);
const clientSelected = computed(() => Boolean(actionBar.value?.clientSelected));
const salespersonSelected = computed(() => Boolean(selectedSalesperson.value));
const canCheckout = computed(
  () =>
    Boolean(calculator.value?.checkoutReady) &&
    (exchangeState.value ? hasItems.value : invoiceTotal.value > 0) &&
    clientSelected.value &&
    salespersonSelected.value,
);
const barcodeValue = ref("");
const barcodeInput = ref<InstanceType<typeof import("vuetify/components").VTextField> | null>(null);
const dialogOpen = ref(false);
let restoringDraft = true;

provide("dialogOpen", dialogOpen);

watch(hasItems, (val) => {
  if (!val) calculator.value?.clearAll();
});

watch(dialogOpen, (v) => {
  if (!v) focusBarcode();
});

watch(noteMenu, (open) => {
  if (open) noteDraft.value = invoiceNotes.value;
});

watch(
  [() => invoiceItems.value?.items, () => actionBar.value?.client, invoiceNotes],
  ([items, client]) => {
    if (restoringDraft || !client) return;
    if (!items?.length && client.id === 0) {
      localStorage.removeItem(INVOICE_DRAFT_KEY);
      return;
    }
    localStorage.setItem(
      INVOICE_DRAFT_KEY,
      JSON.stringify({
        items: items ?? [],
        client,
        notes: invoiceNotes.value,
      } satisfies InvoiceDraft),
    );
  },
  { deep: true },
);

function restoreDraft() {
  try {
    const draft = JSON.parse(
      localStorage.getItem(INVOICE_DRAFT_KEY) || "null",
    ) as InvoiceDraft | null;
    if (!draft) return;
    for (const item of draft.items ?? []) invoiceItems.value?.addItem({ ...item });
    if (draft.client?.id) actionBar.value?.setClient(draft.client);
    invoiceNotes.value = draft.notes ?? "";
    noteDraft.value = invoiceNotes.value;
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
  invoiceNotes.value = "";
  noteDraft.value = "";
  noteMenu.value = false;
  clearDraft();
  actionBar.value?.resetClient();
}

function saveNote() {
  invoiceNotes.value = noteDraft.value.trim();
  noteMenu.value = false;
  focusBarcode();
}

function focusBarcode() {
  barcodeInput.value?.focus();
}
function onBlur() {
  if (!dialogOpen.value) focusBarcode();
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

async function onBarcodeEnter() {
  const value = barcodeValue.value;
  if (!value) return;
  let match: any = null;
  try {
    const p = await GetProductByBarcode(value);
    match = { ...p, qty: 1 };
  } catch {
    match = products.value.find((p) => p.code === value);
  }
  if (match) invoiceItems.value?.addItem(match);
  barcodeValue.value = "";
  focusBarcode();
}

const products = ref<any[]>([]);

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
      discount: i.discount,
      qty: i.qty,
    })),
  );
}

async function onHold() {
  const json = await getItemsJSON();
  if (json === "[]") return;
  await HoldInvoice(json);
  clearInvoice();
}

async function onVoid() {
  const json = await getFullItemsJSON();
  if (json === "[]") return;
  await VoidInvoice(json, "");
  clearInvoice();
}

async function onCheckout() {
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
    { ...payload.returned, category: payload.returned.category ?? "", qty: -1 },
    payload.replacements.map((item) => ({
      ...item,
      category: item.category ?? "",
      qty: Number(item.qty || 1),
    })),
  );
  actionBar.value?.setClient(payload.client);
  exchangeState.value = { originalInvoiceId: payload.originalInvoiceId };
  await refreshNextInvoiceCode("exchange");
  calculator.value?.clearAll();
  snackbar.text = "Exchange loaded. Collect the difference, if any, and check out.";
  snackbar.color = "info";
  snackbar.show = true;
}

async function cancelExchange() {
  clearInvoice();
  calculator.value?.clearAll();
  exchangeState.value = null;
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

function onRestoreHold(id: number, items: any[]) {
  if (!invoiceItems.value) return;
  invoiceItems.value.clearItems();
  for (const item of items) {
    invoiceItems.value.addItem({ ...item, qty: item.qty || 1 });
  }
  DeleteHoldInvoice(id);
}

onMounted(async () => {
  await nextTick();
  restoreDraft();
  focusBarcode();
  try {
    products.value = (await ListProducts()).map((p) => ({ ...p, qty: 1 }));
  } catch (e) {
    console.error("ListProducts failed:", e);
  }
  try {
    const settings = await GetAppSettings();
    taxRate.value = Number(settings?.taxRate ?? 0);
    roundingMode.value = settings?.roundingMode ?? "half_up";
    receiptEnabled.value = Boolean(settings?.receiptEnabled);
    receiptPrinter.value = settings?.receiptPrinter ?? "";
    screenPort.value = settings?.screenPort ?? "";
    branchName.value = settings?.branchName || "Store";
  } catch {}
  try {
    await GetOpenShift();
  } catch {
    try {
      await OpenShift();
    } catch {}
  }
  showCustomerTotal();
  await refreshNextInvoiceCode("sales");
});
</script>

<style>
.app-snackbar .v-snackbar__wrapper {
  background: rgba(var(--v-theme-surface-variant), 0.72) !important;
  color: rgb(var(--v-theme-on-surface-variant)) !important;
  border: 1px solid rgba(var(--v-theme-on-surface-variant), 0.1);
  box-shadow: 0 4px 18px rgba(0, 0, 0, 0.12) !important;
}
</style>
