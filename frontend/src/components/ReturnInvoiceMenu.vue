<template>
  <v-menu v-model="menu" :close-on-content-click="false" location="bottom">
    <template #activator="{ props }">
      <v-btn v-bind="props" class="px-5" variant="tonal">
        <template #prepend><v-icon icon="mdi-autorenew" size="20" class="me-2" /></template>
        Exchange
      </v-btn>
    </template>
    <v-card min-width="430" class="d-flex flex-column">
      <v-card-title>Exchange Invoice</v-card-title>
      <v-card-subtitle>Scan the original sales invoice, then scan the item being returned.</v-card-subtitle>
      <v-card-text>
        <v-text-field v-model="invoiceCode" label="Sales invoice code or barcode" prepend-inner-icon="mdi-barcode" variant="outlined" density="compact" @keydown.enter="loadInvoice" />
        <v-btn block color="primary" variant="tonal" :loading="loading" @click="loadInvoice">Load invoice</v-btn>
        <template v-if="invoice">
          <v-alert class="mt-3" density="compact" type="info">{{ invoice.invoiceCode }}</v-alert>
          <v-text-field v-model="scanValue" class="mt-3" :label="returned ? 'Scan replacement item' : 'Scan returned item'" prepend-inner-icon="mdi-barcode" variant="outlined" density="compact" @keydown.enter="scanItem" />
          <v-list v-if="returned" density="compact" class="pa-0">
            <v-list-item title="Returning" :subtitle="`${returned.name} — $${effectivePrice(returned).toFixed(2)}`" />
            <v-list-item v-if="replacement" title="Replacing with" :subtitle="`${replacement.name} — $${effectivePrice(replacement).toFixed(2)}`" />
          </v-list>
          <v-alert v-if="replacement" class="mt-2" density="compact" :type="effectivePrice(replacement) >= effectivePrice(returned) ? 'success' : 'error'">
            {{ effectivePrice(replacement) >= effectivePrice(returned) ? `Customer pays $${(effectivePrice(replacement) - effectivePrice(returned)).toFixed(2)} at checkout.` : 'Replacement must be the same price or more expensive.' }}
          </v-alert>
        </template>
      </v-card-text>
      <v-card-actions class="pa-3">
        <v-btn class="flex-1-1" variant="text" @click="reset">Cancel</v-btn>
        <v-btn class="flex-1-1" color="primary" :disabled="!returned || !replacement || effectivePrice(replacement) < effectivePrice(returned)" @click="startExchange">Start exchange</v-btn>
      </v-card-actions>
    </v-card>
  </v-menu>
</template>

<script setup lang="ts">
import { inject, ref, watch, type Ref } from "vue";
import { GetExchangeInvoice, GetProductByBarcode, GetProductByCode } from "../../wailsjs/go/main/App";

const emit = defineEmits<{ exchange: [payload: { originalInvoiceId: number; returned: any; replacement: any }] }>();
const dialogOpen = inject<Ref<boolean>>("dialogOpen");
const menu = ref(false);
const loading = ref(false);
const invoiceCode = ref("");
const scanValue = ref("");
const invoice = ref<any>(null);
const returned = ref<any>(null);
const replacement = ref<any>(null);

watch(menu, (open) => {
  if (dialogOpen) dialogOpen.value = open;
});

async function loadInvoice() {
  if (!invoiceCode.value.trim()) return;
  loading.value = true;
  try {
    invoice.value = await GetExchangeInvoice(invoiceCode.value.trim());
    returned.value = null;
    replacement.value = null;
    scanValue.value = "";
  } catch (error: any) { invoice.value = null; alert(error?.toString() || "Invoice not found"); }
  loading.value = false;
}

function findItem(value: string) {
  const normalized = value.trim().toLowerCase();
  return invoice.value?.items?.find((item: any) => item.code.toLowerCase() === normalized || item.barcode?.toLowerCase() === normalized);
}

function effectivePrice(item: any) { return Number(item.price) * (1 - Number(item.discount || 0) / 100); }

async function scanItem() {
  let item = findItem(scanValue.value);
  if (returned.value) {
    const value = scanValue.value.trim();
    let product: any;
    try { product = await GetProductByBarcode(value); }
    catch { try { product = await GetProductByCode(value); } catch { alert("Replacement product not found."); return; } }
    replacement.value = { ...product, price: Number(product.price), discount: Number(product.discount), qty: 1 };
    scanValue.value = "";
    return;
  }
  if (!item) { alert(returned.value ? "Replacement product not found." : "That product is not available on this invoice."); return; }
  if (!returned.value) returned.value = { ...item, price: effectivePrice(item), discount: 0 };
  else replacement.value = { ...item, price: Number(item.price) };
  scanValue.value = "";
}

function startExchange() {
  if (!returned.value || !replacement.value) return;
  emit("exchange", { originalInvoiceId: invoice.value.id, returned: returned.value, replacement: replacement.value });
  menu.value = false;
  reset();
}

function reset() { invoiceCode.value = ""; scanValue.value = ""; invoice.value = null; returned.value = null; replacement.value = null; }
</script>
