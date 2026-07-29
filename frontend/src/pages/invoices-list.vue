<template>
  <div class="invoices-page">
    <v-data-table-server
      v-model:items-per-page="itemsPerPage"
      class="invoices-table"
      :headers="headers"
      :items="items"
      :items-length="totalItems"
      :loading="loading"
      item-value="id"
      hover
      @update:options="loadInvoices"
      @click:row="openInvoiceRow"
    >
      <template #item.invoiceCode="{ item }">
        <span class="font-weight-bold">{{ item.invoiceCode || `Invoice #${item.id}` }}</span>
      </template>
      <template #item.clientId="{ item }">
        {{ item.clientId ? `Client #${item.clientId}` : "Walk-in customer" }}
      </template>
      <template #item.salespersonName="{ item }">
        <v-chip size="small" variant="tonal" prepend-icon="mdi-account-tie">
          {{ item.salespersonName || "Anonymous" }}
        </v-chip>
      </template>
      <template #item.createdAt="{ item }">{{ formatDate(item.createdAt) }}</template>
      <template #item.grandTotal="{ item }">
        <span class="font-weight-bold">${{ money(item.grandTotal) }}</span>
      </template>
      <template #item.actions="{ item }">
        <v-btn
          icon="mdi-printer-outline"
          variant="text"
          density="comfortable"
          :loading="printingId === item.id"
          title="Print receipt again"
          @click.stop="reprint(item)"
        />
        <v-menu location="bottom end">
          <template #activator="{ props }">
            <v-btn v-bind="props" icon="mdi-dots-vertical" variant="text" density="comfortable" @click.stop />
          </template>
          <v-list density="compact" min-width="190">
            <v-list-item prepend-icon="mdi-account-edit" title="Change Client" @click.stop="startChange('client', item)" />
            <v-list-item prepend-icon="mdi-account-tie" title="Change Salesman" @click.stop="startChange('salesperson', item)" />
            <v-list-item v-if="auth.isAdmin" prepend-icon="mdi-delete-outline" title="Delete Invoice" base-color="error" @click.stop="requestDelete(item)" />
          </v-list>
        </v-menu>
      </template>
      <template #no-data>
        <div class="invoices-empty pa-8 text-center">
          <v-icon icon="mdi-cash-register" size="48" class="mb-3 text-medium-emphasis" />
          <div class="text-subtitle-1 font-weight-bold">No invoices found</div>
          <div class="text-body-2 text-medium-emphasis">Completed invoices will appear here.</div>
        </div>
      </template>
    </v-data-table-server>

    <v-dialog v-model="detailOpen" max-width="620">
      <v-card v-if="selected">
        <v-card-title class="d-flex align-center"
          ><div>
            <div class="text-h6">{{ selected.invoiceCode }}</div>
            <div class="text-caption text-medium-emphasis">
              {{ formatDate(selected.createdAt) }}
            </div>
          </div>
          <v-spacer /><v-btn
            icon="mdi-printer-outline"
            variant="text"
            :loading="printingId === selected.id"
            @click="reprint(selected)" /><v-btn
            icon="mdi-close"
            variant="text"
            @click="detailOpen = false"
        /></v-card-title>
        <v-card-text>
          <div class="d-flex flex-wrap ga-2 mb-4">
            <v-chip size="small" variant="tonal" prepend-icon="mdi-account">{{
              selected.clientName
            }}</v-chip
            ><v-chip size="small" variant="tonal" prepend-icon="mdi-account-tie">{{
              selected.salespersonName
            }}</v-chip
            ><v-chip size="small" variant="tonal" prepend-icon="mdi-credit-card">{{
              selected.payments?.map((p: any) => p.accountName).join(", ") || "No settlement"
            }}</v-chip>
          </div>
          <v-list class="pa-0 mb-4" density="compact">
            <v-list-item v-for="line in selected.items" :key="line.code">
              <template #title>{{ line.name }}</template
              ><template #subtitle
                >{{ line.qty }} × ${{ Number(line.price).toFixed(2)
                }}<span v-if="line.discount"> · {{ line.discount }}% discount</span></template
              >
              <template #append
                ><span class="font-weight-bold"
                  >${{ Number(line.lineTotal).toFixed(2) }}</span
                ></template
              >
            </v-list-item>
          </v-list>
          <div class="totals">
            <div>
              <span>Subtotal</span><span>${{ cents(selected.subtotal) }}</span>
            </div>
            <div>
              <span>Discount</span
              ><span class="text-red"
                >-${{ cents(Number(selected.subtotal) - Number(selected.discountedTotal)) }}</span
              >
            </div>
            <v-divider class="my-2" />
            <div class="text-body-1 font-weight-bold">
              <span>Total</span><span>${{ cents(selected.grandTotal) }}</span>
            </div>
          </div>
        </v-card-text>
      </v-card>
    </v-dialog>

    <v-dialog v-model="deleteOpen" max-width="420">
      <v-card
        ><v-card-title>Delete invoice?</v-card-title
        ><v-card-text
          >This permanently deletes {{ deleteInvoice?.invoiceCode || "this invoice" }} and its
          payment records.</v-card-text
        ><v-card-actions
          ><v-spacer /><v-btn variant="text" @click="deleteOpen = false">Cancel</v-btn
          ><v-btn color="error" :loading="deleting" @click="deleteSelected"
            >Delete</v-btn
          ></v-card-actions
        ></v-card
      >
    </v-dialog>
    <v-dialog v-model="changeOpen" max-width="420">
      <v-card
        ><v-card-title>{{ changeTitle }}</v-card-title
        ><v-card-text>
          <v-select
            v-if="changeType === 'client'"
            v-model="changeValue"
            label="Client"
            :items="clients"
            item-title="name"
            item-value="id"
            variant="outlined"
          />
          <v-select
            v-else-if="changeType === 'settlement'"
            v-model="changeValue"
            label="Settlement"
            :items="paymentMethods"
            item-title="name"
            item-value="id"
            variant="outlined"
          />
          <v-select
            v-else
            v-model="changeValue"
            label="Salesman"
            :items="salespersons"
            item-title="fullName"
            item-value="id"
            placeholder="Select a salesman"
            clearable
            variant="outlined"
          /> </v-card-text
        ><v-card-actions
          ><v-spacer /><v-btn variant="text" @click="changeOpen = false">Cancel</v-btn
          ><v-btn
            color="primary"
            :loading="saving"
            :disabled="!changeValue && changeType !== 'salesperson'"
            @click="saveChange"
            >Save</v-btn
          ></v-card-actions
        ></v-card
      >
    </v-dialog>
    <v-snackbar
      v-model="snackbar.show"
      class="app-snackbar"
      timeout="3500"
      location="top"
      variant="flat"
      >{{ snackbar.text }}</v-snackbar
    >
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from "vue";
import {
  ChangeInvoiceClient,
  ChangeInvoiceSalesperson,
  ChangeInvoiceSettlement,
  DeleteInvoice,
  GetInvoiceDetails,
  ListClients,
  ListInvoicesPage,
  ListPaymentMethods,
  ListSalespersons,
  ReprintInvoice,
} from "../../wailsjs/go/main/App";
import { useAuthStore } from "../stores/auth";

const items = ref<any[]>([]),
  totalItems = ref(0),
  itemsPerPage = ref(20),
  loading = ref(true),
  currentPage = ref(1);
const auth = useAuthStore();
const selected = ref<any>(null),
  detailOpen = ref(false),
  changeOpen = ref(false),
  saving = ref(false),
  printingId = ref<number | null>(null);
const deleteOpen = ref(false),
  deleting = ref(false),
  deleteInvoice = ref<any>(null);
const changeType = ref<"client" | "settlement" | "salesperson">("client"),
  changeValue = ref<number | null>(null),
  changeInvoice = ref<any>(null);
const clients = ref<any[]>([]),
  paymentMethods = ref<any[]>([]),
  salespersons = ref<any[]>([]);
const snackbar = reactive({ show: false, text: "", color: "success" });
const changeTitle = computed(() =>
  changeType.value === "client"
    ? "Change Client"
    : changeType.value === "settlement"
      ? "Change Settlement"
      : "Change Salesman",
);
function money(value: number | undefined) {
  return ((Number(value) || 0) / 100).toFixed(2);
}
function cents(value: number | undefined) {
  return ((Number(value) || 0) / 100).toFixed(2);
}
function formatDate(value?: string) {
  return value?.slice(0, 19) || "Date unavailable";
}
const headers = [
  { title: "Invoice", key: "invoiceCode" },
  { title: "Customer", key: "clientId" },
  { title: "Salesman", key: "salespersonName" },
  { title: "Date", key: "createdAt" },
  { title: "Total", key: "grandTotal", align: "end" as const },
  { title: "", key: "actions", sortable: false, align: "end" as const },
];
async function openInvoice(item: any) {
  try {
    selected.value = await GetInvoiceDetails(item.id);
    detailOpen.value = true;
  } catch {
    notify("Could not load invoice details", "error");
  }
}
function openInvoiceRow(_event: any, row: { item: any }) {
  openInvoice(row.item);
}
async function reprint(item: any) {
  printingId.value = item.id;
  try {
    await ReprintInvoice(item.id);
    notify("Receipt sent to the default receipt printer");
  } catch (e: any) {
    notify(e?.toString() || "Could not print receipt", "error");
  } finally {
    printingId.value = null;
  }
}
async function startChange(type: "client" | "settlement" | "salesperson", item: any) {
  await ensureEditOptions();
  changeType.value = type;
  changeInvoice.value = item;
  changeValue.value =
    type === "client"
      ? item.clientId
      : type === "salesperson"
        ? item.salespersonId || null
        : item.cashboxId;
  changeOpen.value = true;
}
function requestDelete(item: any) {
  deleteInvoice.value = item;
  deleteOpen.value = true;
}
async function deleteSelected() {
  if (!deleteInvoice.value || !auth.isAdmin) return;
  deleting.value = true;
  try {
    await DeleteInvoice(deleteInvoice.value.id);
    deleteOpen.value = false;
    notify("Invoice deleted");
    await refresh();
  } catch (e: any) {
    notify(e?.toString() || "Could not delete invoice", "error");
  } finally {
    deleting.value = false;
  }
}
async function saveChange() {
  if (!changeInvoice.value) return;
  saving.value = true;
  try {
    if (changeType.value === "client")
      await ChangeInvoiceClient(changeInvoice.value.id, changeValue.value || 0);
    else if (changeType.value === "settlement")
      await ChangeInvoiceSettlement(changeInvoice.value.id, changeValue.value || 0);
    else await ChangeInvoiceSalesperson(changeInvoice.value.id, changeValue.value || 0);
    await openInvoice(changeInvoice.value);
    changeOpen.value = false;
    await refresh();
    notify("Invoice updated");
  } catch (e: any) {
    notify(e?.toString() || "Could not update invoice", "error");
  } finally {
    saving.value = false;
  }
}
function notify(text: string, color = "success") {
  snackbar.text = text;
  snackbar.color = color;
  snackbar.show = true;
}
async function loadInvoices({ page, itemsPerPage: requestedItemsPerPage }: { page: number; itemsPerPage: number }) {
  loading.value = true;
  currentPage.value = page;
  try {
    const result = await ListInvoicesPage(page, requestedItemsPerPage);
    items.value = result.items ?? [];
    totalItems.value = result.total ?? 0;
  } catch {
    items.value = [];
    totalItems.value = 0;
  } finally {
    loading.value = false;
  }
}

async function refresh() {
  await loadInvoices({ page: currentPage.value, itemsPerPage: itemsPerPage.value });
}

let editOptionsPromise: Promise<void> | null = null;
async function ensureEditOptions() {
  if (!editOptionsPromise) {
    editOptionsPromise = Promise.all([ListClients(), ListPaymentMethods(), ListSalespersons()])
      .then(([loadedClients, loadedPaymentMethods, loadedSalespersons]) => {
        clients.value = loadedClients;
        paymentMethods.value = loadedPaymentMethods;
        salespersons.value = loadedSalespersons;
      })
      .catch(() => {
        editOptionsPromise = null;
        throw new Error("Could not load invoice edit options");
      });
  }
  return editOptionsPromise;
}
</script>

<style scoped>
.invoices-page {
  height: 100%;
  min-height: 0;
}
.invoices-scroll {
  flex: 1 1 auto;
  min-height: 0;
}
.invoice-item {
  border-bottom: 1px solid rgba(var(--v-border-color), 0.08);
  cursor: pointer;
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
.totals > div {
  display: flex;
  justify-content: space-between;
  margin: 4px 0;
}
.app-snackbar .v-snackbar__wrapper {
  background: rgba(var(--v-theme-surface-variant), 0.72) !important;
  color: rgb(var(--v-theme-on-surface-variant)) !important;
  border: 1px solid rgba(var(--v-theme-on-surface-variant), 0.1);
  box-shadow: 0 4px 18px rgba(0, 0, 0, 0.12) !important;
}
</style>
