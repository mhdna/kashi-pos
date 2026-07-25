<template>
  <div class="d-flex h-100" style="width: 100%; overflow: hidden">
    <div class="d-flex flex-column" style="flex: 5 5 0; min-width: 0; min-height: 0">
      <div>
        <v-card-title class="d-flex justify-space-between">
          <div>{{ exchangeState ? "Exchange" : "Invoice" }}</div>
          <div class="d-flex">
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
        <v-card-subtitle> {{ invoiceCode || "------" }} </v-card-subtitle>
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
      <div style="flex: 3 3 0; min-width: 0">
        <Calculator ref="calculator" :total="invoiceTotal" />
      </div>
      <div style="flex: 2 2 0; min-width: 0">
        <ActionBar
          ref="actionBar"
          :total="invoiceTotal"
          :can-checkout="canCheckout"
          :has-items="hasItems"
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

  <v-snackbar v-model="snackbar.show" :color="snackbar.color" timeout="4000" location="top right">
    {{ snackbar.text }}
  </v-snackbar>
</template>

<script lang="ts" setup>
import { ref, provide, watch, onMounted, computed, reactive } from "vue";
import {
  GetProductByBarcode,
  ListProducts,
  HoldInvoice,
  VoidInvoice,
  CheckoutInvoice,
  CheckoutExchangeInvoice,
  GetAppSettings,
  DeleteHoldInvoice,
  GetOpenShift,
  OpenShift,
} from "../../wailsjs/go/main/App";
import ActionBar from "@/components/ActionBar.vue";
import Calculator from "@/components/Calculator.vue";
import InvoiceItems from "@/components/InvoiceItems.vue";
import InvoiceSummary from "@/components/InvoiceSummary.vue";
import OnHoldMenu from "@/components/OnHoldMenu.vue";
import ProductSearchDialog from "@/components/ProductSearchDialog.vue";
import ReturnInvoiceMenu from "@/components/ReturnInvoiceMenu.vue";
import type { ExchangeSelection } from "@/lib/exchange";

const taxRate = ref(0);
const roundingMode = ref("half_up");
const receiptEnabled = ref(false);
const receiptPrinter = ref("");
const branchName = ref("Store");
const invoiceCode = ref("");
const exchangeState = ref<{ originalInvoiceId: number; sourceInvoiceCode: string } | null>(null);
const invoiceItems = ref<InstanceType<typeof InvoiceItems> | null>(null);
const calculator = ref<InstanceType<typeof Calculator> | null>(null);
const actionBar = ref<InstanceType<typeof ActionBar> | null>(null);

function roundCents(value: number, mode: string): number {
  if (mode === "down") return Math.floor(value + 1e-9);
  if (mode === "up") return Math.ceil(value - 1e-9);
  return Math.floor(value + 0.5);
}

const invoiceTotal = computed(() => {
  const discountedCents = invoiceItems.value?.discountedSubtotalCents ?? 0;
  const taxCents = roundCents((discountedCents / 100) * taxRate.value * 100, roundingMode.value);
  return (discountedCents + taxCents) / 100;
});
const hasItems = computed(() => (invoiceItems.value?.items?.length ?? 0) > 0);
const canCheckout = computed(() => calculator.value?.diff === 0 && (exchangeState.value ? hasItems.value : invoiceTotal.value > 0));
const barcodeValue = ref("");
const barcodeInput = ref<InstanceType<typeof import("vuetify/components").VTextField> | null>(null);
const dialogOpen = ref(false);

provide("dialogOpen", dialogOpen);

watch(hasItems, (val) => {
  if (!val) calculator.value?.clearAll();
});

watch(dialogOpen, (v) => {
  if (!v) focusBarcode();
});

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

async function getItemsJSON(): Promise<string> {
  const items = invoiceItems.value?.items ?? [];
  return JSON.stringify(items.map((i: any) => ({ code: i.code, qty: i.qty })));
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
  invoiceItems.value?.clearItems();
}

async function onVoid() {
  const json = await getFullItemsJSON();
  if (json === "[]") return;
  await VoidInvoice(json, "");
  invoiceItems.value?.clearItems();
}

async function onCheckout() {
  const json = await getFullItemsJSON();
  if (json === "[]") return;
  const isExchange = Boolean(exchangeState.value);
  printing.value = receiptEnabled.value && !isExchange;
  try {
    const paymentsJSON = calculator.value?.paymentsJSON ?? "[]";
    const clientID = actionBar.value?.clientId ?? 0;
    const result = exchangeState.value
      ? await CheckoutExchangeInvoice(json, paymentsJSON, exchangeState.value.originalInvoiceId, clientID)
      : await CheckoutInvoice(json, paymentsJSON, clientID, branchName.value, branchName.value, "", "", receiptPrinter.value);
    invoiceCode.value = result?.invoiceCode ?? "";
    snackbar.text = result?.receiptError
      ? `Invoice created; receipt failed — ${result.receiptError}`
      : isExchange
        ? `Exchange completed — ${result?.invoiceCode ?? ""}`
        : receiptEnabled.value
        ? `Receipt printed — ${result?.invoiceCode ?? ""}`
        : `Invoice created — ${result?.invoiceCode ?? ""}`;
    snackbar.color = "success";
    snackbar.show = true;
    invoiceItems.value?.clearItems();
    exchangeState.value = null;
    if (isExchange) actionBar.value?.resetClient();
  } catch (e: any) {
    snackbar.text = e?.toString() || "Checkout failed";
    snackbar.color = "red-darken-4";
    snackbar.show = true;
  }
  printing.value = false;
}

function onExchange(payload: ExchangeSelection) {
  invoiceItems.value?.clearItems();
  invoiceItems.value?.addExchangeItems(
    { ...payload.returned, category: payload.returned.category ?? "", qty: -1 },
    payload.replacements.map((item) => ({
      ...item,
      category: item.category ?? "",
      qty: Number(item.qty || 1),
    })),
  );
  actionBar.value?.setClient(payload.client);
  invoiceCode.value = payload.sourceInvoiceCode;
  exchangeState.value = {
    originalInvoiceId: payload.originalInvoiceId,
    sourceInvoiceCode: payload.sourceInvoiceCode,
  };
  calculator.value?.clearAll();
  snackbar.text = "Exchange loaded. Collect the difference, if any, and check out.";
  snackbar.color = "info";
  snackbar.show = true;
}

function cancelExchange() {
  invoiceItems.value?.clearItems();
  calculator.value?.clearAll();
  exchangeState.value = null;
  actionBar.value?.resetClient();
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
    branchName.value = settings?.branchName || "Store";
  } catch {}
  try {
    await GetOpenShift();
  } catch {
    try {
      await OpenShift();
    } catch {}
  }
});
</script>
