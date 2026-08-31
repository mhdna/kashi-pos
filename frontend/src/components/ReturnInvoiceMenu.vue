<template>
  <v-btn v-if="!hideActivator" class="px-5" variant="tonal" @click="menu = true">
    <template #prepend>
      <v-icon icon="mdi-autorenew" size="20" class="me-2" />
    </template>
    Exchange
  </v-btn>

  <v-dialog
    v-model="menu"
    :max-width="invoice ? 780 : 360"
    @after-enter="focusInvoiceInput"
    @after-leave="clearForm"
  >
    <v-card class="exchange-dialog-card d-flex flex-column">
      <v-card-title>Exchange Invoice</v-card-title>
      <v-card-subtitle v-if="!invoice">Load the original sale to begin</v-card-subtitle>

      <v-card-text
        class="exchange-dialog-layout"
        :class="{ 'exchange-dialog-layout--entry': !invoice }"
      >
        <div v-if="invoice" class="exchange-dialog-content">
          <div class="exchange-input-row">
            <div class="exchange-input-control">
              <v-text-field
                v-model="invoiceCode"
                density="compact"
                placeholder="Sales invoice code or barcode"
                prepend-inner-icon="mdi-barcode"
                variant="outlined"
                disabled
              />
            </div>
            <v-btn height="40" variant="text" @click="changeInvoice">Change</v-btn>
          </div>

          <v-alert class="mb-3" density="compact" type="success" variant="tonal">
            {{ invoice.invoiceCode }} · {{ invoice.client.name }} —
            {{ invoice.items.length }} available
            {{ invoice.items.length === 1 ? "item" : "items" }}
          </v-alert>

          <v-card class="mb-3" variant="outlined">
            <v-list-subheader>Items available to return</v-list-subheader>
            <v-list density="compact" class="exchange-available-items py-0">
              <v-list-item
                v-for="item in invoice.items"
                :key="`${item.code}-${item.price}-${item.discount}`"
                :active="Boolean(selectedReturn(item))"
                active-color="primary"
                :title="item.name"
                :subtitle="`${item.code} · ${item.barcode}${item.color ? ` · ${item.color}` : ''}${item.size ? ` · ${item.size}` : ''} · ${item.available} available`"
                @click="selectReturned(item)"
              >
                <template #prepend>
                  <v-icon icon="mdi-barcode" class="me-3" />
                </template>
                <template #append>
                  <div class="exchange-item-selection">
                    <span
                      v-if="selectedReturn(item)"
                      class="text-primary text-caption font-weight-medium"
                    >
                      Selected ×{{ selectedReturn(item)?.qty ?? 1 }}
                    </span>
                    <span class="text-body-2 font-weight-medium">
                      ${{ effectivePrice(item).toFixed(2) }}
                    </span>
                    <v-btn
                      v-if="selectedReturn(item)"
                      aria-label="Remove returned item"
                      icon="mdi-delete"
                      size="x-small"
                      variant="text"
                      @click.stop="removeReturned(item.code, item.price, item.discount)"
                    />
                  </div>
                </template>
              </v-list-item>
            </v-list>
          </v-card>

          <div class="exchange-input-control">
            <v-text-field
              ref="scanInput"
              v-model="scanValue"
              density="compact"
              placeholder="Scan or enter product being returned"
              prepend-inner-icon="mdi-barcode"
              variant="outlined"
              :error-messages="scanError"
              @keydown.enter.prevent="scanItem"
              @update:model-value="scanError = ''"
            />
          </div>
        </div>

        <div v-if="!invoice" class="exchange-inline-numpad">
          <div class="exchange-inline-numpad-label">Invoice code</div>
          <div class="exchange-inline-numpad-display">
            <v-text-field
              ref="invoiceInput"
              v-model="invoiceCode"
              autofocus
              class="exchange-inline-numpad-display-input"
              hide-details
              inputmode="numeric"
              placeholder="0"
              variant="plain"
              @keydown.enter.prevent="loadInvoice"
              @update:model-value="invoiceError = ''"
            />
          </div>
          <div v-if="invoiceError" class="exchange-inline-numpad-error text-error">
            {{ invoiceError }}
          </div>
          <div class="exchange-inline-numpad-grid">
            <v-btn
              v-for="digit in exchangePadDigits"
              :key="digit"
              class="exchange-inline-numpad-key"
              size="small"
              variant="tonal"
              :disabled="exchangePadDisabled"
              @click="appendExchangeDigit(digit)"
            >
              {{ digit }}
            </v-btn>
            <v-btn
              aria-label="Clear exchange input"
              class="exchange-inline-numpad-key"
              size="small"
              variant="text"
              :disabled="exchangePadDisabled"
              @click="clearExchangeInput"
            >
              C
            </v-btn>
            <v-btn
              class="exchange-inline-numpad-key"
              size="small"
              variant="tonal"
              :disabled="exchangePadDisabled"
              @click="appendExchangeDigit('0')"
            >
              0
            </v-btn>
            <v-btn
              aria-label="Delete last exchange digit"
              class="exchange-inline-numpad-key"
              size="small"
              variant="text"
              :disabled="exchangePadDisabled"
              @click="backspaceExchangeInput"
            >
              ⌫
            </v-btn>
          </div>
          <v-btn
            block
            class="exchange-load-button mt-2"
            color="primary"
            height="48"
            size="small"
            variant="flat"
            :disabled="exchangePadDisabled || !activeExchangeInput.trim()"
            :loading="exchangePadDisabled"
            @click="submitExchangeInput"
          >
            Load
          </v-btn>
        </div>
      </v-card-text>

      <v-card-actions class="pa-3">
        <v-btn class="flex-1-1" variant="text" @click="menu = false">Cancel</v-btn>
        <v-btn class="flex-1-1" color="primary" :disabled="!returned.length" @click="startExchange">
          Start exchange
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import { computed, inject, ref, watch, type Ref } from "vue";
import { useExchangeDialog } from "@/composables/useExchangeDialog";
import { effectiveExchangePrice, type ExchangeSelection } from "@/lib/exchange";

const emit = defineEmits<{
  exchange: [payload: ExchangeSelection];
}>();
const props = defineProps<{ roundingMode?: string; hideActivator?: boolean }>();
const roundingMode = computed(() => props.roundingMode ?? "half_up");

const dialogOpen = inject<Ref<boolean> | null>("dialogOpen", null);
const menu = ref(false);
const exchangePadDigits = ["1", "2", "3", "4", "5", "6", "7", "8", "9"];

defineExpose({
  open: () => {
    menu.value = true;
  },
  close: () => {
    menu.value = false;
  },
});
const {
  loading,
  invoiceCode,
  scanValue,
  invoice,
  returned,
  invoiceError,
  scanError,
  invoiceInput,
  scanInput,
  focusInvoiceInput,
  loadInvoice,
  selectReturned,
  scanItem,
  removeReturned,
  changeInvoice,
  createSelection,
  clearForm,
} = useExchangeDialog(() => props.roundingMode ?? "half_up");
const activeExchangeInput = computed(() => invoiceCode.value);
const exchangePadDisabled = computed(() => loading.value);

watch(menu, (open) => {
  if (dialogOpen) dialogOpen.value = open;
});

function effectivePrice(item: Parameters<typeof effectiveExchangePrice>[0]): number {
  return effectiveExchangePrice(item, props.roundingMode ?? "half_up");
}

function selectedReturn(item: Parameters<typeof effectiveExchangePrice>[0]) {
  return returned.value.find(
    (selected) =>
      selected.code === item.code &&
      (selected.soldUnitPrice ?? selected.price) === item.price &&
      (selected.soldUnitDiscount ?? selected.discount) === item.discount,
  );
}

function appendExchangeDigit(digit: string) {
  if (exchangePadDisabled.value) return;
  if (invoice.value) {
    if (scanValue.value.length < 80) scanValue.value += digit;
    scanError.value = "";
  } else {
    if (invoiceCode.value.length < 64) invoiceCode.value += digit;
    invoiceError.value = "";
  }
}

function clearExchangeInput() {
  if (invoice.value) {
    scanValue.value = "";
    scanError.value = "";
  } else {
    invoiceCode.value = "";
    invoiceError.value = "";
  }
}

function backspaceExchangeInput() {
  if (invoice.value) {
    scanValue.value = scanValue.value.slice(0, -1);
    scanError.value = "";
  } else {
    invoiceCode.value = invoiceCode.value.slice(0, -1);
    invoiceError.value = "";
  }
}

function submitExchangeInput() {
  if (invoice.value) scanItem();
  else void loadInvoice();
}

function startExchange() {
  const selection = createSelection();
  if (!selection) return;
  emit("exchange", selection);
  menu.value = false;
}
</script>

<style scoped>
.exchange-input-row,
.exchange-input-control {
  align-items: flex-start;
  display: flex;
  gap: 6px;
}

.exchange-input-control {
  flex: 1 1 auto;
  min-width: 0;
}

.exchange-input-control :deep(.v-input) {
  flex: 1 1 auto;
  min-width: 0;
}

.exchange-dialog-card {
  height: min(640px, calc(100dvh - 32px));
}

.exchange-dialog-layout {
  display: grid;
  flex: 1 1 auto;
  gap: 16px;
  grid-template-columns: minmax(0, 1fr);
  min-height: 0;
  overflow-y: auto;
}

.exchange-dialog-layout--entry {
  place-items: center;
}

.exchange-dialog-content {
  min-width: 0;
}

.exchange-item-selection {
  align-items: center;
  display: flex;
  gap: 8px;
}

.exchange-available-items {
  height: 192px;
  overflow-y: auto;
}

.exchange-available-items :deep(.v-list-item) {
  min-height: 64px;
}

.exchange-inline-numpad {
  width: min(100%, 280px);
}

.exchange-inline-numpad-label {
  font-size: var(--kashi-font-size-sm);
  opacity: 0.7;
  text-align: center;
}

.exchange-inline-numpad-display {
  min-height: 52px;
}

.exchange-inline-numpad-display-input :deep(input) {
  font-size: var(--kashi-font-size-2xl);
  font-weight: 700;
  text-align: center;
}

.exchange-inline-numpad-error {
  font-size: var(--kashi-font-size-sm);
  margin-bottom: 8px;
  text-align: center;
}

.exchange-inline-numpad-grid {
  display: grid;
  gap: 6px;
  grid-auto-rows: 56px;
  grid-template-columns: repeat(3, 1fr);
}

.exchange-inline-numpad-key {
  border: 1px solid rgba(var(--v-border-color), 0.18);
  border-radius: 4px;
  font-size: 2rem !important;
  height: 100%;
  min-width: 0;
  padding: 0;
  transition:
    border-color 120ms ease,
    background-color 120ms ease;
}

.exchange-inline-numpad-key:hover {
  border-color: currentColor;
}

.exchange-load-button {
  font-size: var(--kashi-font-size-lg) !important;
}
</style>
