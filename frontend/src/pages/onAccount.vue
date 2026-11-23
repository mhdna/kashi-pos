<template>
  <div class="records-page">
    <div class="records-head">
      <div class="records-title">
        <h1>{{ t("On Account") }}</h1>
      </div>
      <k-spacer />
      <k-select
        v-model="month"
        class="on-account-month"
        :items="monthChoices"
        item-title="title"
        item-value="value"
        density="compact"
        variant="outlined"
        hide-details
        data-test="on-account-month"
        @update:model-value="load"
      />
      <k-text-field
        v-model="search"
        class="on-account-search"
        :placeholder="t('Search client or phone')"
        prepend-inner-icon="mdi-magnify"
        density="compact"
        variant="outlined"
        hide-details
        clearable
        data-test="on-account-search"
      />
    </div>

    <div class="on-account-totals" data-test="on-account-totals">
      <div class="on-account-total">
        <span class="on-account-total-label">{{ t("Sold") }}</span>
        <span class="on-account-total-value" data-test="total-sold">{{ formatUsd(totals.total / 100) }}</span>
      </div>
      <div class="on-account-total">
        <span class="on-account-total-label">{{ t("Collected") }}</span>
        <span class="on-account-total-value" data-test="total-paid">{{ formatUsd(totals.paid / 100) }}</span>
      </div>
      <div class="on-account-total on-account-total--owed">
        <span class="on-account-total-label">{{ t("Owed") }}</span>
        <span class="on-account-total-value" data-test="total-owed">{{ formatUsd(totals.outstanding / 100) }}</span>
      </div>
    </div>

    <div v-if="loadError" class="text-error pb-2" data-test="on-account-error">{{ t("Could not load: {error}", { error: loadError }) }}</div>

    <k-data-table-virtual
      class="records-table"
      :headers="headers"
      :items="rows"
      :loading="loading"
      height="100%"
      :item-height="56"
      fixed-header
      item-value="id"
      hover
      :no-data-text="search ? t('No client matches.') : t('Nobody owes on account.')"
      @click:row="(_e: unknown, row: any) => openDetail(row.item)"
    >
      <template #item.name="{ item }">
        <span class="font-weight-medium">{{ item.name }}</span>
      </template>
      <template #item.phone="{ item }">
        <span v-if="item.phone" dir="ltr">{{ item.phone }}</span>
        <span v-else class="text-medium-emphasis">{{ t("No phone") }}</span>
      </template>
      <template #item.total="{ item }">{{ formatUsd(item.total / 100) }}</template>
      <template #item.paid="{ item }">{{ formatUsd(item.paid / 100) }}</template>
      <template #item.outstanding="{ item }">
        <span class="font-weight-bold">{{ formatUsd(item.outstanding / 100) }}</span>
      </template>
      <template #item.lastSaleAt="{ item }">{{ formatServerDateTime(item.lastSaleAt) }}</template>
    </k-data-table-virtual>

    <k-dialog v-model="detailOpen" max-width="760">
      <k-card v-if="selected">
        <k-card-title class="d-flex align-center">
          <div>
            <div class="text-h6">{{ selected.name }}</div>
            <div v-if="selected.phone" class="text-caption text-medium-emphasis" dir="ltr">{{ selected.phone }}</div>
          </div>
          <k-spacer />
          <span class="owes-chip" data-test="detail-balance">{{ t("Owes {amount}", { amount: formatUsd(selected.outstanding / 100) }) }}</span>
          <k-btn icon="mdi-close" variant="text" :aria-label="t('Close')" @click="detailOpen = false" />
        </k-card-title>
        <k-progress-linear v-if="detailLoading" indeterminate />
        <k-card-text v-else>
          <k-table density="compact">
            <thead>
              <tr>
                <th>{{ t("Invoice") }}</th>
                <th>{{ t("Date") }}</th>
                <th class="text-end">{{ t("Charged") }}</th>
                <th class="text-end">{{ t("Paid") }}</th>
                <th class="text-end">{{ t("Due") }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="entry in openEntries" :key="entry.id" data-test="detail-entry">
                <td>{{ entry.invoiceCode }}</td>
                <td>{{ formatServerDateTime(entry.createdAt) }}</td>
                <td class="text-end">{{ formatUsd(Number(entry.amount) / 100) }}</td>
                <td class="text-end">{{ formatUsd(Number(entry.paid) / 100) }}</td>
                <td class="text-end font-weight-bold">{{ formatUsd(Number(entry.outstanding) / 100) }}</td>
              </tr>
              <tr v-if="!openEntries.length">
                <td colspan="5" class="text-center text-medium-emphasis py-4">{{ t("Nothing owed.") }}</td>
              </tr>
            </tbody>
          </k-table>
        </k-card-text>
      </k-card>
    </k-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { t } from "@/i18n";
import { GetClientAccountDetails, ListOnAccountClients, ListOnAccountMonths } from "../../wailsjs/go/main/App";
import { formatUsd } from "@/lib/format";
import { formatServerDateTime } from "@/lib/datetime";
import { currentMonth, defaultMonth, filterOnAccountClients, monthChoices as buildMonthChoices, onAccountTotals, type OnAccountClient } from "@/lib/onAccount";

// Computed so the column titles follow the language toggle.
const headers = computed(() => [
  { title: t("Client"), key: "name", sortable: false },
  { title: t("Phone"), key: "phone", sortable: false },
  { title: t("Invoices"), key: "invoiceCount", sortable: false, align: "end" as const },
  { title: t("Sold"), key: "total", sortable: false, align: "end" as const },
  { title: t("Paid"), key: "paid", sortable: false, align: "end" as const },
  { title: t("Owes"), key: "outstanding", sortable: false, align: "end" as const },
  { title: t("Last sale"), key: "lastSaleAt", sortable: false },
]);

const clients = ref<OnAccountClient[]>([]);
const loading = ref(false);
const loadError = ref("");
const search = ref("");
// The picker offers only months where someone still owes. It opens on this
// month when it has debts, else the newest month that does.
const month = ref(currentMonth());
const monthChoices = ref(buildMonthChoices([]));
const rows = computed(() => filterOnAccountClients(clients.value, search.value ?? ""));
// The numbers over the table follow the rows shown, so a search narrows them too.
const totals = computed(() => onAccountTotals(rows.value));

async function load() {
  loading.value = true;
  try {
    const result = await ListOnAccountClients(month.value);
    clients.value = (result?.clients ?? []) as OnAccountClient[];
    loadError.value = "";
  } catch (error) {
    clients.value = [];
    loadError.value = String(error);
  } finally {
    loading.value = false;
  }
}
onMounted(async () => {
  try {
    const months = ((await ListOnAccountMonths()) ?? []) as string[];
    monthChoices.value = buildMonthChoices(months);
    month.value = defaultMonth(months);
  } catch {
    // Without the months the picker keeps All months and this month's list.
  }
  await load();
});

const detailOpen = ref(false);
const detailLoading = ref(false);
const selected = ref<OnAccountClient | null>(null);
const entries = ref<any[]>([]);
const openEntries = computed(() => entries.value.filter((entry) => Number(entry.outstanding) > 0));

async function openDetail(client: OnAccountClient) {
  selected.value = client;
  entries.value = [];
  detailOpen.value = true;
  detailLoading.value = true;
  try {
    const details = await GetClientAccountDetails(client.id);
    entries.value = (details?.entries ?? []) as any[];
  } finally {
    detailLoading.value = false;
  }
}
</script>

<style scoped>
@import "@/styles/records-page.css";

.on-account-month {
  max-width: 190px;
}
.owes-chip {
  margin-inline: 8px;
  color: rgb(var(--k-theme-error));
  font-weight: 700;
}
.on-account-search {
  max-width: 280px;
}
.on-account-totals {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  padding-bottom: 12px;
}
.on-account-total {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 10px 14px;
}
.on-account-total-label {
  font-size: 0.8rem;
  opacity: 0.7;
}
.on-account-total-value {
  font-size: 1.25rem;
  font-weight: 700;
}
.on-account-total--owed .on-account-total-value {
  color: rgb(var(--k-theme-error));
}
</style>
