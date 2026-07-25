<template>
  <div class="invoices-page">
    <!-- <v-card class="invoices-card" variant="flat"> -->
    <!-- <v-card-title class="d-flex align-center ga-3 pa-5">
        <v-avatar color="primary" variant="tonal" size="40">
          <v-icon icon="mdi-cash-register" size="22" />
        </v-avatar>
        <div>
          <div class="text-h6 font-weight-bold">Invoices</div>
          <div class="text-caption text-medium-emphasis">
            Browse completed sales and payment totals.
          </div>
        </div>
        <v-spacer />
        <v-chip v-if="!loading" color="primary" variant="tonal" size="small">
          {{ items.length }} {{ items.length === 1 ? "invoice" : "invoices" }}
        </v-chip>
      </v-card-title> -->

    <v-divider />
    <v-progress-linear v-if="loading" indeterminate color="primary" />

    <v-infinite-scroll
      v-else-if="items.length"
      class="invoices-scroll"
      height="100%"
      side="end"
      empty-text="All invoices loaded"
      @load="loadMore"
    >
      <v-list class="pa-0" lines="two">
        <v-list-item v-for="item in visibleItems" :key="item.id" class="invoice-item py-3 px-5">
          <template #prepend>
            <v-avatar color="primary" variant="tonal" size="36" class="me-3">
              <v-icon icon="mdi-cash-register" size="19" />
            </v-avatar>
          </template>

          <template #title>
            <span class="font-weight-bold">{{ item.invoiceCode || `Invoice #${item.id}` }}</span>
          </template>
          <template #subtitle>
            <span>{{ item.clientId ? `Client #${item.clientId}` : "Walk-in customer" }}</span>
            <span class="mx-1">·</span>
            <span>{{ formatDate(item.createdAt) }}</span>
            <span class="mx-1">·</span>
            <span>Subtotal ${{ money(item.subtotal) }}</span>
            <span v-if="item.discount" class="text-primary">
              · Discount ${{ money(item.discountedTotal) }}
            </span>
          </template>

          <template #append>
            <div class="invoice-total text-right">
              <div class="text-subtitle-1 font-weight-bold">${{ money(item.grandTotal) }}</div>
              <div class="text-caption text-medium-emphasis">Total</div>
            </div>
          </template>
        </v-list-item>
      </v-list>
    </v-infinite-scroll>

    <div v-else class="invoices-empty pa-8 text-center">
      <v-icon icon="mdi-cash-register" size="48" class="mb-3 text-medium-emphasis" />
      <div class="text-subtitle-1 font-weight-bold">No invoices found</div>
      <div class="text-body-2 text-medium-emphasis">Completed invoices will appear here.</div>
    </div>
    <!-- </v-card> -->
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ListInvoices } from "../../wailsjs/go/main/App";

const pageSize = 20;
const items = ref<any[]>([]);
const visibleCount = ref(pageSize);
const loading = ref(true);

const visibleItems = computed(() => items.value.slice(0, visibleCount.value));

function money(value: number | undefined) {
  return ((Number(value) || 0) / 100).toFixed(2);
}

function formatDate(value?: string) {
  return value?.slice(0, 19) || "Date unavailable";
}

function loadMore({ done }: { done: (status: "ok" | "empty" | "error") => void }) {
  if (visibleCount.value >= items.value.length) {
    done("empty");
    return;
  }
  visibleCount.value = Math.min(visibleCount.value + pageSize, items.value.length);
  done("ok");
}

onMounted(async () => {
  try {
    const invoices = (await ListInvoices()) as any[];
    items.value = [...(invoices ?? [])].sort((a, b) => Number(b.id) - Number(a.id));
  } catch {
    items.value = [];
  } finally {
    loading.value = false;
  }
});
</script>

<style scoped>
.invoices-page {
  /*box-sizing: border-box;*/
  height: 100%;
  min-height: 0;
  /*padding: 16px;*/
}

/* .invoices-card {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
} */

.invoices-scroll {
  flex: 1 1 auto;
  min-height: 0;
}

.invoice-item {
  border-bottom: 1px solid rgba(var(--v-border-color), 0.08);
}

.invoice-item:last-child {
  border-bottom: 0;
}

.invoice-total {
  min-width: 100px;
}

.invoices-empty {
  margin: auto;
}
</style>
