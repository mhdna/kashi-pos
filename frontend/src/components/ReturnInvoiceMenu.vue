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
            {{ invoice.invoiceCode }} · {{ invoice.client.name }} — {{ invoice.items.length }} available
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
const props = defineProps<{ roundingMode?: string }>();
const roundingMode = computed(() => props.roundingMode ?? "half_up");

const dialogOpen = inject<Ref<boolean>>("dialogOpen");
const menu = ref(false);
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

watch(menu, (open) => {
  if (dialogOpen) dialogOpen.value = open;
});

function effectivePrice(item: Parameters<typeof effectiveExchangePrice>[0]): number {
  return effectiveExchangePrice(item, props.roundingMode ?? "half_up");
}

function startExchange() {
  const selection = createSelection();
  if (!selection) return;
  emit("exchange", selection);
  menu.value = false;
}
</script>
