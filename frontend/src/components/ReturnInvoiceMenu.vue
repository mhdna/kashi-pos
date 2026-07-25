<template>
  <v-btn class="px-5" variant="tonal" @click="menu = true">
    <template #prepend>
      <v-icon icon="mdi-autorenew" size="20" class="me-2" />
    </template>
    Exchange
  </v-btn>

  <v-dialog
    v-model="menu"
    max-width="560"
    @after-enter="focusInvoiceInput"
    @after-leave="clearForm"
  >
    <v-card class="d-flex flex-column">
      <v-card-title>Exchange Invoice</v-card-title>
      <v-card-subtitle>
        Load the original sale, scan the returned item, then scan its replacement.
      </v-card-subtitle>

      <v-card-text>
        <div class="d-flex ga-2 align-start">
          <v-text-field
            ref="invoiceInput"
            v-model="invoiceCode"
            autofocus
            class="flex-grow-1"
            density="compact"
            label="Sales invoice code or barcode"
            prepend-inner-icon="mdi-barcode"
            variant="outlined"
            :disabled="Boolean(invoice)"
            :error-messages="invoiceError"
            @keydown.enter.prevent="loadInvoice"
            @update:model-value="invoiceError = ''"
          />
          <v-btn
            v-if="!invoice"
            color="primary"
            height="40"
            variant="tonal"
            :loading="loading"
            @click="loadInvoice"
          >
            Load
          </v-btn>
          <v-btn v-else height="40" variant="text" @click="changeInvoice">Change</v-btn>
        </div>

        <template v-if="invoice">
          <v-alert class="mb-3" density="compact" type="success" variant="tonal">
            {{ invoice.invoiceCode }} loaded — {{ invoice.items.length }} available
            {{ invoice.items.length === 1 ? "item" : "items" }}
          </v-alert>

          <v-card class="mb-3" variant="outlined">
            <v-list-subheader>Items available to return</v-list-subheader>
            <v-list density="compact" class="py-0" style="max-height: 220px; overflow-y: auto">
              <v-list-item
                v-for="item in invoice.items"
                :key="item.code"
                :active="returned?.code === item.code"
                active-color="primary"
                :title="item.name"
                :subtitle="`${item.code} · ${item.barcode} · ${item.available} available`"
                @click="selectReturned(item)"
              >
                <template #prepend>
                  <v-icon icon="mdi-barcode" class="me-3" />
                </template>
                <template #append>
                  <span class="text-body-2 font-weight-medium">
                    ${{ effectivePrice(item).toFixed(2) }}
                  </span>
                </template>
              </v-list-item>
            </v-list>
          </v-card>

          <v-text-field
            ref="scanInput"
            v-model="scanValue"
            density="compact"
            :label="returned ? 'Scan replacement product' : 'Scan product being returned'"
            prepend-inner-icon="mdi-barcode"
            variant="outlined"
            :loading="scanning"
            :error-messages="scanError"
            @keydown.enter.prevent="scanItem"
            @update:model-value="scanError = ''"
          />

          <v-list v-if="returned" density="compact" class="pa-0">
            <v-list-item
              title="Returning"
              :subtitle="`${returned.name} · ${returned.code}`"
            >
              <template #append>
                <span class="text-red-darken-2 font-weight-medium">
                  −${{ effectivePrice(returned).toFixed(2) }}
                </span>
              </template>
            </v-list-item>
            <v-list-item
              v-if="replacement"
              title="Replacement"
              :subtitle="`${replacement.name} · ${replacement.code}`"
            >
              <template #append>
                <span class="text-green-darken-2 font-weight-medium">
                  +${{ effectivePrice(replacement).toFixed(2) }}
                </span>
              </template>
            </v-list-item>
          </v-list>

          <v-alert v-if="replacement" class="mt-2" density="compact" type="info" variant="tonal">
            Customer pays
            ${{ exchangeDifference().toFixed(2) }}
            plus applicable tax at checkout.
          </v-alert>
        </template>
      </v-card-text>

      <v-card-actions class="pa-3">
        <v-btn class="flex-1-1" variant="text" @click="menu = false">Cancel</v-btn>
        <v-btn
          class="flex-1-1"
          color="primary"
          :disabled="!returned || !replacement"
          @click="startExchange"
        >
          Start exchange
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import { inject, nextTick, ref, watch, type Ref } from "vue";
import { GetExchangeInvoice, GetProductByBarcode, GetProductByCode } from "../../wailsjs/go/main/App";

interface ExchangeItem {
  code: string;
  barcode: string;
  name: string;
  price: number;
  discount: number;
  available?: number;
  qty?: number;
}

interface ExchangeInvoice {
  id: number;
  invoiceCode: string;
  items: ExchangeItem[];
}

const emit = defineEmits<{
  exchange: [payload: { originalInvoiceId: number; returned: ExchangeItem; replacement: ExchangeItem }];
}>();

const dialogOpen = inject<Ref<boolean>>("dialogOpen");
const menu = ref(false);
const loading = ref(false);
const scanning = ref(false);
const invoiceCode = ref("");
const scanValue = ref("");
const invoice = ref<ExchangeInvoice | null>(null);
const returned = ref<ExchangeItem | null>(null);
const replacement = ref<ExchangeItem | null>(null);
const invoiceError = ref("");
const scanError = ref("");
const invoiceInput = ref<InstanceType<typeof import("vuetify/components").VTextField> | null>(null);
const scanInput = ref<InstanceType<typeof import("vuetify/components").VTextField> | null>(null);

watch(menu, (open) => {
  if (dialogOpen) dialogOpen.value = open;
});

function normalize(value: string): string {
  return value.trim().toLowerCase();
}

function effectivePrice(item: ExchangeItem): number {
  return Number(item.price) * (1 - Number(item.discount || 0) / 100);
}

function exchangeDifference(): number {
  if (!returned.value || !replacement.value) return 0;
  return effectivePrice(replacement.value) - effectivePrice(returned.value);
}

function focusInvoiceInput() {
  nextTick(() => invoiceInput.value?.focus());
}

function focusScanInput() {
  nextTick(() => scanInput.value?.focus());
}

async function loadInvoice() {
  const code = invoiceCode.value.trim();
  if (!code || loading.value) return;
  loading.value = true;
  invoiceError.value = "";
  try {
    const loaded = (await GetExchangeInvoice(code)) as ExchangeInvoice;
    invoice.value = {
      ...loaded,
      items: (loaded.items ?? []).map((item) => ({
        ...item,
        price: Number(item.price),
        discount: Number(item.discount),
        available: Number(item.available),
      })),
    };
    returned.value = null;
    replacement.value = null;
    scanValue.value = "";
    focusScanInput();
  } catch (error: any) {
    invoice.value = null;
    invoiceError.value = error?.toString() || "Sales invoice not found.";
    focusInvoiceInput();
  } finally {
    loading.value = false;
  }
}

function findInvoiceItem(value: string): ExchangeItem | undefined {
  const scanned = normalize(value);
  return invoice.value?.items.find(
    (item) => normalize(item.code) === scanned || normalize(item.barcode || "") === scanned,
  );
}

function selectReturned(item: ExchangeItem) {
  returned.value = {
    ...item,
    price: effectivePrice(item),
    discount: 0,
    qty: -1,
  };
  replacement.value = null;
  scanError.value = "";
  scanValue.value = "";
  focusScanInput();
}

async function findProduct(value: string): Promise<ExchangeItem> {
  try {
    return (await GetProductByBarcode(value)) as ExchangeItem;
  } catch {
    return (await GetProductByCode(value)) as ExchangeItem;
  }
}

async function scanItem() {
  const value = scanValue.value.trim();
  if (!value || scanning.value) return;
  scanError.value = "";

  if (!returned.value) {
    const item = findInvoiceItem(value);
    if (!item) {
      scanError.value = "This product is not available to return on the loaded invoice.";
      scanValue.value = "";
      focusScanInput();
      return;
    }
    selectReturned(item);
    return;
  }

  scanning.value = true;
  try {
    const product = await findProduct(value);
    const candidate: ExchangeItem = {
      ...product,
      price: Number(product.price),
      discount: Number(product.discount),
      qty: 1,
    };
    if (normalize(candidate.code) === normalize(returned.value.code)) {
      throw new Error("Choose a different product as the replacement.");
    }
    if (effectivePrice(candidate) < effectivePrice(returned.value)) {
      throw new Error("Replacement must be the same price or more expensive.");
    }
    replacement.value = candidate;
    scanValue.value = "";
  } catch (error: any) {
    replacement.value = null;
    scanValue.value = "";
    scanError.value = error?.toString() || "Replacement product not found.";
  } finally {
    scanning.value = false;
    focusScanInput();
  }
}

function startExchange() {
  if (!invoice.value || !returned.value || !replacement.value) return;
  emit("exchange", {
    originalInvoiceId: invoice.value.id,
    returned: returned.value,
    replacement: replacement.value,
  });
  menu.value = false;
}

function changeInvoice() {
  invoice.value = null;
  returned.value = null;
  replacement.value = null;
  scanValue.value = "";
  scanError.value = "";
  invoiceError.value = "";
  focusInvoiceInput();
}

function clearForm() {
  invoiceCode.value = "";
  invoice.value = null;
  returned.value = null;
  replacement.value = null;
  scanValue.value = "";
  invoiceError.value = "";
  scanError.value = "";
  loading.value = false;
  scanning.value = false;
}
</script>
