<template>
  <div class="salespersons-page pa-4">
    <div class="d-flex align-center mb-4 ga-3">
      <h2 class="text-h6 font-weight-bold">Salespersons</h2>
      <v-spacer />
      <template v-if="auth.isAdmin">
        <v-text-field
          v-model="fromDate"
          type="date"
          label="From"
          variant="outlined"
          density="compact"
          hide-details
          style="max-width: 170px"
          @change="refresh"
        />
        <v-text-field
          v-model="toDate"
          type="date"
          label="To"
          variant="outlined"
          density="compact"
          hide-details
          style="max-width: 170px"
          @change="refresh"
        />
      </template>
      <span v-else class="text-caption text-medium-emphasis">{{ rangeLabel }}</span>
      <v-btn icon="mdi-refresh" variant="text" :loading="loading" @click="refresh" />
    </div>

    <section class="mb-6">
      <div class="d-flex align-center mb-2">
        <span class="text-subtitle-1 font-weight-bold">Performance</span>
        <v-spacer />
        <span class="text-caption text-medium-emphasis">{{ rows.length }} salespersons</span>
      </div>
      <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-2" />
      <v-list class="pa-0" bg-color="transparent">
        <v-list-item v-for="row in rows" :key="row.salespersonId" class="performance-item mb-2" rounded="lg">
          <template #prepend>
            <InitialsAvatar :name="row.salespersonName" class="me-3" />
          </template>
          <div class="performance-item__main">
            <div class="d-flex align-center mb-1">
              <span class="font-weight-bold">{{ row.salespersonName }}</span>
              <v-spacer />
              <span class="text-body-2 text-medium-emphasis">{{ money(row.netAmount) }}</span>
              <span class="font-weight-bold ms-3" style="min-width: 40px; text-align: right">{{ Math.round(row.score) }}</span>
            </div>
            <v-progress-linear :model-value="row.score" height="10" rounded color="primary" />
          </div>
        </v-list-item>
        <v-list-item v-if="!rows.length && !loading" class="text-medium-emphasis">
          No sales recorded for this range.
        </v-list-item>
      </v-list>
    </section>

    <v-table class="salespersons-table">
      <thead>
        <tr>
          <th>Salesperson</th>
          <th class="text-right">Amount Sold</th>
          <th class="text-right">Invoices</th>
          <th class="text-right">Items Sold</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in rows" :key="row.salespersonId">
          <td>{{ row.salespersonName }}</td>
          <td class="text-right">{{ money(row.netAmount) }}</td>
          <td class="text-right">{{ row.invoiceCount }}</td>
          <td class="text-right">{{ row.itemsSold }}</td>
        </tr>
        <tr v-if="!rows.length && !loading">
          <td colspan="4" class="text-medium-emphasis">No sales recorded for this range.</td>
        </tr>
      </tbody>
    </v-table>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ListSalespersonSalesSummary } from "../../wailsjs/go/main/App";
import { useAuthStore } from "@/stores/auth";
import InitialsAvatar from "@/components/InitialsAvatar.vue";

type SummaryRow = {
  salespersonId: number;
  salespersonName: string;
  invoiceCount: number;
  itemsSold: number;
  grossAmount: number;
  returnAmount: number;
  netAmount: number;
  discountAmount: number;
  subtotalAmount: number;
};

type ScoredRow = SummaryRow & { score: number };

const auth = useAuthStore();

function monthStart() {
  const now = new Date();
  return new Date(now.getFullYear(), now.getMonth(), 1).toISOString().slice(0, 10);
}
function today() {
  return new Date().toISOString().slice(0, 10);
}

const fromDate = ref(monthStart());
const toDate = ref(today());
const loading = ref(false);
const rows = ref<ScoredRow[]>([]);

const rangeLabel = computed(() => `${fromDate.value} – ${toDate.value}`);

function money(cents: number) {
  return `$${((Number(cents) || 0) / 100).toFixed(2)}`;
}

/**
 * Composite 0-100 performance score, normalized against the best
 * performer in the fetched range on each sub-metric: net sales (55%),
 * average basket size (20%), return rate inverted (15%), discount rate
 * inverted (10%). A sub-metric contributes full marks to everyone when
 * nobody in the range triggers it (e.g. no returns at all).
 */
function scoreRows(summary: SummaryRow[]): ScoredRow[] {
  const withBasket = summary.map((row) => ({
    ...row,
    avgBasket: row.invoiceCount > 0 ? row.netAmount / row.invoiceCount : 0,
    returnRate: row.grossAmount > 0 ? row.returnAmount / row.grossAmount : 0,
    discountRate: row.subtotalAmount > 0 ? row.discountAmount / row.subtotalAmount : 0,
  }));
  const maxNet = Math.max(0, ...withBasket.map((r) => r.netAmount));
  const maxBasket = Math.max(0, ...withBasket.map((r) => r.avgBasket));
  const maxReturnRate = Math.max(0, ...withBasket.map((r) => r.returnRate));
  const maxDiscountRate = Math.max(0, ...withBasket.map((r) => r.discountRate));

  return withBasket
    .map((row) => {
      const netScore = maxNet > 0 ? (Math.max(0, row.netAmount) / maxNet) * 100 : 100;
      const basketScore = maxBasket > 0 ? (row.avgBasket / maxBasket) * 100 : 100;
      const returnScore = maxReturnRate > 0 ? (1 - row.returnRate / maxReturnRate) * 100 : 100;
      const discountScore = maxDiscountRate > 0 ? (1 - row.discountRate / maxDiscountRate) * 100 : 100;
      const score = netScore * 0.55 + basketScore * 0.2 + returnScore * 0.15 + discountScore * 0.1;
      return { ...row, score: Math.max(0, Math.min(100, score)) };
    })
    .sort((a, b) => b.score - a.score);
}

async function refresh() {
  loading.value = true;
  try {
    const summary = (await ListSalespersonSalesSummary(fromDate.value, toDate.value)) as SummaryRow[];
    rows.value = scoreRows(summary);
  } catch {
    rows.value = [];
  } finally {
    loading.value = false;
  }
}

onMounted(refresh);
</script>

<style scoped>
.salespersons-page {
  height: 100%;
  overflow: auto;
}
.performance-item {
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
}
.performance-item__main {
  width: 100%;
}
.salespersons-table {
  background: transparent !important;
}
</style>
