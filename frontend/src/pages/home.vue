<template>
  <div class="d-flex h-100" style="width: 100%; overflow: hidden">
    <div class="d-flex flex-column" style="flex: 5 5 0; min-width: 0; min-height: 0">
      <div>
        <v-card-title class="d-flex justify-space-between">
          <div>Invoice</div>
          <div class="d-flex">
            <div class="me-2">
              <OnHoldMenu @restore="onRestoreHold" />
            </div>
            <ReturnInvoiceMenu />
          </div>
        </v-card-title>
        <v-card-subtitle> SA/CN/2026/1441 </v-card-subtitle>
      </div>
      <div class="d-flex align-center mx-4 mt-4">
        <v-text-field
          ref="barcodeInput"
          v-model="barcodeValue"
          class="flex-grow-1"
          density="compact"
          hide-details="auto"
          placeholder="Scan or Enter Barcode"
          variant="outlined"
          @blur="onBlur"
          @keydown.enter="onBarcodeEnter"
        />
        <v-btn icon size="small" variant="text" class="ms-1" @click="onBarcodeEnter">
          <v-icon icon="mdi-magnify" size="20" />
        </v-btn>
        <ProductSearchDialog @select="onProductSelect" />
      </div>
      <InvoiceItems ref="invoiceItems" style="flex: 1 1 0; min-height: 0" />
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
          :total="invoiceTotal"
          :can-checkout="canCheckout"
          :has-items="hasItems"
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
  GetAppSettings,
  DeleteHoldInvoice,
} from "../../wailsjs/go/main/App";
import ActionBar from "@/components/ActionBar.vue";
import Calculator from "@/components/Calculator.vue";
import InvoiceItems from "@/components/InvoiceItems.vue";
import InvoiceSummary from "@/components/InvoiceSummary.vue";
import OnHoldMenu from "@/components/OnHoldMenu.vue";
import ProductSearchDialog from "@/components/ProductSearchDialog.vue";
import ReturnInvoiceMenu from "@/components/ReturnInvoiceMenu.vue";

const taxRate = ref(0);
const invoiceItems = ref<InstanceType<typeof InvoiceItems> | null>(null);
const calculator = ref<InstanceType<typeof Calculator> | null>(null);
const invoiceTotal = computed(() => {
  const subtotal = invoiceItems.value?.subtotal ?? 0;
  const totalDiscount = invoiceItems.value?.totalDiscount ?? 0;
  return (subtotal - totalDiscount) * (1 + taxRate.value);
});
const canCheckout = computed(() => calculator.value?.diff === 0 && invoiceTotal.value > 0);
const hasItems = computed(() => (invoiceItems.value?.items?.length ?? 0) > 0);
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
  printing.value = true;
  try {
    const settings = await GetAppSettings();
    const receiptNo = "INV-" + Date.now();
    await CheckoutInvoice(
      json,
      settings.branchName || "Store",
      settings.branchName || "Branch",
      "",
      "",
      receiptNo,
      settings.receiptPrinter || "",
    );
    snackbar.text = "Receipt printed successfully";
    snackbar.color = "success";
    snackbar.show = true;
    invoiceItems.value?.clearItems();
  } catch (e: any) {
    snackbar.text = e?.toString() || "Print failed";
    snackbar.color = "red-darken-4";
    snackbar.show = true;
  }
  printing.value = false;
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
    taxRate.value = settings?.taxRate ?? 0;
  } catch {}
});
</script>
