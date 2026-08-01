<template>
  <v-btn v-if="!hideActivator" class="px-5" variant="tonal" @click="menu = true">
    <template #prepend>
      <v-icon icon="mdi-autorenew" size="20" class="me-2" />
    </template>
    Exchange
  </v-btn>

  <v-dialog
    v-model="menu"
    max-width="780"
    @after-enter="focusInvoiceInput"
    @after-leave="clearForm"
  >
    <v-card class="d-flex flex-column">
      <v-card-title>Exchange Invoice</v-card-title>
      <v-card-subtitle>
        Load the original sale, scan the returned item, then scan its replacement.
      </v-card-subtitle>

      <v-card-text class="exchange-dialog-layout">
        <div class="exchange-dialog-content">
        <div class="exchange-input-row">
          <div class="exchange-input-control">
            <v-text-field
              ref="invoiceInput"
              v-model="invoiceCode"
              autofocus
              density="compact"
              placeholder="Sales invoice code or barcode"
              prepend-inner-icon="mdi-barcode"
              variant="outlined"
              :disabled="Boolean(invoice)"
              :error-messages="invoiceError"
              @keydown.enter.prevent="loadInvoice"
              @update:model-value="invoiceError = ''"
            />
          </div>
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
            {{ invoice.invoiceCode }} · {{ invoice.client.name }} — {{ invoice.items.length }} available
            {{ invoice.items.length === 1 ? "item" : "items" }}
          </v-alert>

          <v-card class="mb-3" variant="outlined">
            <v-list-subheader>Items available to return</v-list-subheader>
            <v-list density="compact" class="py-0" style="max-height: 220px; overflow-y: auto">
              <v-list-item
                v-for="item in invoice.items"
                :key="`${item.code}-${item.price}-${item.discount}`"
                :active="
                  returned?.code === item.code &&
                  (returned?.soldUnitPrice ?? returned?.price) === item.price &&
                  (returned?.soldUnitDiscount ?? returned?.discount) === item.discount
                "
                active-color="primary"
                :title="item.name"
                :subtitle="`${item.code} · ${item.barcode}${item.color ? ` · ${item.color}` : ''}${item.size ? ` · ${item.size}` : ''} · ${item.available} available`"
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

          <div class="exchange-input-control">
            <v-text-field
              ref="scanInput"
              v-model="scanValue"
              density="compact"
              :placeholder="returned ? 'Scan replacement product' : 'Scan product being returned'"
              prepend-inner-icon="mdi-barcode"
              variant="outlined"
              :loading="scanning"
              :error-messages="scanError"
              @keydown.enter.prevent="scanItem"
              @update:model-value="scanError = ''"
            />
          </div>

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
              v-for="item in replacements"
              :key="item.code"
              title="Replacement"
              :subtitle="`${item.name} · ${item.code} · Qty ${item.qty}`"
            >
              <template #append>
                <span class="text-green-darken-2 font-weight-medium">
                  +${{ exchangeLineTotal(item, roundingMode).toFixed(2) }}
                </span>
                <v-btn
                  class="ms-2"
                  icon="mdi-delete"
                  size="x-small"
                  variant="text"
                  @click="removeReplacement(item.code)"
                />
              </template>
            </v-list-item>
          </v-list>

          <v-alert
            v-if="replacements.length"
            class="mt-2"
            density="compact"
            :type="hasEnoughReplacementValue ? 'success' : 'warning'"
            variant="tonal"
          >
            <template v-if="hasEnoughReplacementValue">
              Replacement total: ${{ replacementsTotal.toFixed(2) }}. Customer pays
              ${{ exchangeDifference.toFixed(2) }} plus applicable tax at checkout.
            </template>
            <template v-else>
              Add ${{ remainingReplacementValue.toFixed(2) }} more in replacement products.
            </template>
          </v-alert>
        </template>
        </div>

        <div class="exchange-inline-numpad">
          <div class="exchange-inline-numpad-label">
            {{ invoice ? "Product barcode" : "Invoice code" }}
          </div>
          <div class="exchange-inline-numpad-display">
            {{ activeExchangeInput || "0" }}
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
            class="mt-2"
            color="primary"
            size="small"
            variant="flat"
            :disabled="exchangePadDisabled || !activeExchangeInput.trim()"
            :loading="exchangePadDisabled"
            @click="submitExchangeInput"
          >
            {{ invoice ? "Scan" : "Load" }}
          </v-btn>
        </div>
      </v-card-text>

      <v-card-actions class="pa-3">
        <v-btn class="flex-1-1" variant="text" @click="menu = false">Cancel</v-btn>
        <v-btn
          class="flex-1-1"
          color="primary"
          :disabled="!returned || !hasEnoughReplacementValue"
          @click="startExchange"
        >
          Start exchange
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import { computed, inject, ref, watch, type Ref } from "vue";
import { useExchangeDialog } from "@/composables/useExchangeDialog";
import { effectiveExchangePrice, exchangeLineTotal, type ExchangeSelection } from "@/lib/exchange";

const emit = defineEmits<{
  exchange: [payload: ExchangeSelection];
}>();
const props = defineProps<{ roundingMode?: string; hideActivator?: boolean }>();
const roundingMode = computed(() => props.roundingMode ?? "half_up");

const dialogOpen = inject<Ref<boolean>>("dialogOpen");
const menu = ref(false);
const exchangePadDigits = ["1", "2", "3", "4", "5", "6", "7", "8", "9"];

defineExpose({ open: () => { menu.value = true; }, close: () => { menu.value = false; } });
const {
  loading,
  scanning,
  invoiceCode,
  scanValue,
  invoice,
  returned,
  replacements,
  invoiceError,
  scanError,
  invoiceInput,
  scanInput,
  replacementsTotal,
  exchangeDifference,
  remainingReplacementValue,
  hasEnoughReplacementValue,
  focusInvoiceInput,
  loadInvoice,
  selectReturned,
  scanItem,
  removeReplacement,
  changeInvoice,
  createSelection,
  clearForm,
} = useExchangeDialog(() => props.roundingMode ?? "half_up");
const activeExchangeInput = computed(() => invoice.value ? scanValue.value : invoiceCode.value);
const exchangePadDisabled = computed(() => loading.value || scanning.value);

watch(menu, (open) => {
  if (dialogOpen) dialogOpen.value = open;
});

function effectivePrice(item: Parameters<typeof effectiveExchangePrice>[0]): number {
  return effectiveExchangePrice(item, props.roundingMode ?? "half_up");
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
  if (invoice.value) void scanItem();
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

.exchange-dialog-layout {
  display: grid;
  gap: 20px;
  grid-template-columns: minmax(0, 1fr) 176px;
}

.exchange-dialog-content {
  min-width: 0;
}

.exchange-inline-numpad {
  align-self: start;
  width: 176px;
}

.exchange-inline-numpad-label {
  font-size: 0.75rem;
  opacity: 0.7;
  text-align: center;
}

.exchange-inline-numpad-display {
  font-size: 1.15rem;
  font-weight: 700;
  min-height: 40px;
  overflow: hidden;
  padding: 6px 4px;
  text-align: center;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.exchange-inline-numpad-grid {
  display: grid;
  gap: 4px;
  grid-template-columns: repeat(3, 1fr);
}

.exchange-inline-numpad-key {
  font-size: 1rem;
  height: 36px;
  min-width: 0;
  padding: 0;
}

@media (max-width: 650px) {
  .exchange-dialog-layout {
    grid-template-columns: 1fr;
  }

  .exchange-inline-numpad {
    justify-self: center;
  }
}
</style>
