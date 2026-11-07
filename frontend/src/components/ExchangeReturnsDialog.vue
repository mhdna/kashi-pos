<template>
  <!-- The exchange screen's "Returned items": the sale's items with how many
       of each the exchange returns, changed in place (lib/exchangeReturns.ts).
       Each change goes to the cart at once, and the quote prices it again. -->
  <k-dialog
    :model-value="modelValue"
    max-width="560"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <k-card class="exchange-returns d-flex flex-column" rounded="lg">
      <k-card-title class="d-flex align-center pa-4">
        <k-icon icon="mdi-swap-horizontal" class="me-2" />
        <div class="flex-grow-1">
          <div class="text-subtitle-1 font-weight-bold">Returned items</div>
          <div v-if="sale" class="text-caption text-medium-emphasis">
            From {{ sale.invoiceCode }}
          </div>
        </div>
      </k-card-title>
      <k-divider />
      <k-card-text class="exchange-returns-body pa-0">
        <div v-if="loading" class="pa-6 text-center text-medium-emphasis">Loading the sale…</div>
        <div v-else-if="loadError" class="pa-6 text-center text-error" data-test="returns-error">
          {{ loadError }}
        </div>
        <div
          v-for="row in rows"
          v-else
          :key="row.key"
          class="exchange-returns-row"
          :class="{ 'exchange-returns-row--returned': row.returned > 0 }"
          :data-key="row.key"
        >
          <div class="exchange-returns-text">
            <div class="text-body-2 font-weight-medium" dir="auto">{{ row.item.name }}</div>
            <div class="text-caption text-medium-emphasis">
              {{ row.item.code }}{{ variant(row) }} · {{ formatCents(unitCents(row)) }} each ·
              {{ row.available }} on the sale
              <span v-if="row.item.deal">
                · {{ row.item.freeQty ? `${row.item.freeQty} free, ` : ""
                }}{{ row.item.deal }}</span
              >
            </div>
          </div>
          <div class="exchange-returns-stepper">
            <k-btn
              icon="mdi-minus"
              size="small"
              variant="tonal"
              aria-label="Return one fewer"
              :disabled="Boolean(problemFor(row, row.returned - 1))"
              @click="change(row, row.returned - 1)"
            />
            <span class="exchange-returns-qty" data-test="returned-qty">{{ row.returned }}</span>
            <k-btn
              icon="mdi-plus"
              size="small"
              variant="tonal"
              aria-label="Return one more"
              :disabled="Boolean(problemFor(row, row.returned + 1))"
              @click="change(row, row.returned + 1)"
            />
          </div>
          <div class="exchange-returns-state text-caption" data-test="returned-state">
            {{ row.returned > 0 ? "Returned" : "Not returned" }}
          </div>
        </div>
        <div
          v-if="!loading && !loadError && !rows.length"
          class="pa-6 text-center text-medium-emphasis"
        >
          Nothing is left to return on this sale.
        </div>
      </k-card-text>
      <div
        v-if="hint"
        class="exchange-returns-hint text-caption px-4 pt-2"
        data-test="returns-hint"
      >
        {{ hint }}
      </div>
      <k-card-actions class="pa-3">
        <k-spacer />
        <k-btn variant="flat" color="primary" @click="emit('update:modelValue', false)">Done</k-btn>
      </k-card-actions>
    </k-card>
  </k-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { GetExchangeInvoiceByID } from "../../wailsjs/go/main/App";
import { effectiveExchangePrice, mergeDealUnits, type ExchangeItem } from "@/lib/exchange";
import { formatCents } from "@/lib/exchangeQuote";
import {
  returnQtyProblem,
  returnRows,
  setReturnedQty,
  type ReturnRow,
} from "@/lib/exchangeReturns";

const props = defineProps<{
  modelValue: boolean;
  originalInvoiceId: number;
  // The cart's lines as they are now.
  items: any[];
  roundingMode?: string;
}>();
const emit = defineEmits<{
  "update:modelValue": [open: boolean];
  // The cart with the returns changed, to put in place of the old one.
  change: [items: any[]];
}>();

const loading = ref(false);
const loadError = ref("");
const sale = ref<{ invoiceCode: string; items: ExchangeItem[] } | null>(null);
const hint = ref("");

async function load() {
  loading.value = true;
  loadError.value = "";
  hint.value = "";
  try {
    const loaded: any = await GetExchangeInvoiceByID(props.originalInvoiceId);
    sale.value = {
      invoiceCode: String(loaded.invoiceCode ?? ""),
      items: mergeDealUnits(
        (loaded.items ?? []).map((item: any) => ({
          ...item,
          price: Number(item.price),
          discount: Number(item.discount),
          available: Number(item.available),
        })),
      ),
    };
  } catch (e: any) {
    sale.value = null;
    loadError.value = e?.toString() || "The sale couldn't be loaded.";
  } finally {
    loading.value = false;
  }
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) void load();
  },
  { immediate: true },
);

const rows = computed<ReturnRow[]>(() =>
  sale.value ? returnRows(sale.value.items, props.items) : [],
);

function problemFor(row: ReturnRow, qty: number): string {
  return returnQtyProblem(rows.value, row.key, qty);
}

function change(row: ReturnRow, qty: number) {
  const problem = problemFor(row, qty);
  if (problem) {
    hint.value = problem;
    return;
  }
  hint.value = "";
  emit(
    "change",
    setReturnedQty(props.items, sale.value!.items, row.key, qty, props.roundingMode ?? "half_up"),
  );
}

function unitCents(row: ReturnRow): number {
  return Math.round(effectiveExchangePrice(row.item, props.roundingMode ?? "half_up") * 100);
}

function variant(row: ReturnRow): string {
  const parts = [row.item.color, row.item.size].filter(Boolean);
  return parts.length ? ` · ${parts.join(" / ")}` : "";
}
</script>

<style scoped>
.exchange-returns-body {
  max-height: 60vh;
  overflow-y: auto;
}
.exchange-returns-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto 92px;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  border-bottom: 1px solid rgba(var(--k-theme-on-surface), 0.08);
}
.exchange-returns-row--returned {
  background: rgba(var(--k-theme-primary), 0.06);
}
.exchange-returns-text {
  min-width: 0;
}
.exchange-returns-stepper {
  display: flex;
  align-items: center;
  gap: 8px;
}
.exchange-returns-qty {
  min-width: 24px;
  text-align: center;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.exchange-returns-state {
  text-align: right;
  color: rgba(var(--k-theme-on-surface), 0.7);
}
.exchange-returns-hint {
  color: rgb(var(--k-theme-error));
}
</style>
