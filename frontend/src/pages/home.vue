<template>
  <div class="d-flex" style="width: 100%; overflow: hidden">
    <div style="flex: 5 5 0; min-width: 0">
      <v-sheet>
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
          <v-text-field ref="barcodeInput" v-model="barcodeValue" class="flex-grow-1" density="compact"
            hide-details="auto" placeholder="Scan or Enter Barcode" variant="outlined" @blur="onBlur"
            @keydown.enter="onBarcodeEnter" />
          <v-btn icon size="small" variant="text" class="ms-1" @click="onBarcodeEnter">
            <v-icon icon="mdi-magnify" size="20" />
          </v-btn>
          <ProductSearchDialog @select="onProductSelect" />
        </div>
        <InvoiceItems ref="invoiceItems" />
        <InvoiceSummary :subtotal="invoiceItems?.subtotal ?? 0" :total-discount="invoiceItems?.totalDiscount ?? 0" />
      </v-sheet>
    </div>
    <div style="flex: 3 3 0; min-width: 0">
      <Calculator :total="invoiceTotal" />
    </div>
    <div class="me-2" style="flex: 2 2 0; min-width: 0">
      <ActionBar :total="invoiceTotal" @hold="onHold" @void="onVoid" @checkout="onCheckout" />
    </div>
  </div>

  <v-dialog v-model="showReceipt" max-width="320">
    <v-img v-if="receiptPath" :src="'file://' + receiptPath" width="320" />
  </v-dialog>
</template>

<script lang="ts" setup>
import { ref, provide, watch, onMounted, computed } from "vue";
import { GetProductByBarcode, ListProducts, HoldInvoice, VoidInvoice, CheckoutInvoice, GetAppSettings } from "../../wailsjs/go/main/App";
import ActionBar from "@/components/ActionBar.vue";
import Calculator from "@/components/Calculator.vue";
import InvoiceItems from "@/components/InvoiceItems.vue";
import InvoiceSummary from "@/components/InvoiceSummary.vue";
import OnHoldMenu from "@/components/OnHoldMenu.vue";
import ProductSearchDialog from "@/components/ProductSearchDialog.vue";
import ReturnInvoiceMenu from "@/components/ReturnInvoiceMenu.vue";

const invoiceItems = ref<InstanceType<typeof InvoiceItems> | null>(null);
const invoiceTotal = computed(() => {
  const subtotal = invoiceItems.value?.subtotal ?? 0;
  const totalDiscount = invoiceItems.value?.totalDiscount ?? 0;
  return (subtotal - totalDiscount) * 1.11;
});
const barcodeValue = ref("");
const barcodeInput = ref<InstanceType<typeof import("vuetify/components").VTextField> | null>(null);
const dialogOpen = ref(false);

provide("dialogOpen", dialogOpen);

watch(dialogOpen, (v) => { if (!v) focusBarcode(); });

function focusBarcode() { barcodeInput.value?.focus(); }
function onBlur() { if (!dialogOpen.value) focusBarcode(); }

function onProductSelect(product: { name: string; code: string; category: string; price: number; discount: number; qty: number }) {
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

const showReceipt = ref(false);
const receiptPath = ref("");

async function getItemsJSON(): Promise<string> {
  const items = invoiceItems.value?.items ?? [];
  return JSON.stringify(items.map((i: any) => ({ code: i.code, qty: i.qty })));
}

async function getFullItemsJSON(): Promise<string> {
  const items = invoiceItems.value?.items ?? [];
  return JSON.stringify(items.map((i: any) => ({
    name: i.name, code: i.code, category: i.category,
    price: i.price, discount: i.discount, qty: i.qty,
  })));
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
  const settings = await GetAppSettings();
  const receiptNo = "INV-" + Date.now();
  const path = await CheckoutInvoice(json,
    settings.branchName || "Store", settings.branchName || "Branch",
    "", "", receiptNo, settings.receiptPrinter || "",
  );
  if (path) {
    receiptPath.value = path;
    showReceipt.value = true;
    setTimeout(() => { showReceipt.value = false; }, 2000);
  }
  invoiceItems.value?.clearItems();
}

function onRestoreHold(items: any[]) {
  if (!invoiceItems.value) return;
  invoiceItems.value.clearItems();
  for (const item of items) {
    invoiceItems.value.addItem({ ...item, qty: item.qty || 1 });
  }
}

onMounted(async () => {
  focusBarcode();
  try {
    products.value = (await ListProducts()).map((p) => ({ ...p, qty: 1 }));
  } catch (e) { console.error("ListProducts failed:", e) }
});
</script>
