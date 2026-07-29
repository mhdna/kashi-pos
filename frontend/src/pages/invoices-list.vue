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
        <v-menu v-model="actionMenus[item.id]" location="bottom end">
          <template #activator="{ props }">
            <v-btn v-bind="props" icon="mdi-dots-vertical" variant="text" density="comfortable" @click.stop />
          </template>
          <v-list density="compact" min-width="190">
            <v-list-item prepend-icon="mdi-account-edit" title="Change Client" @click.stop="startChange('client', item)" />
            <v-list-item prepend-icon="mdi-account-tie" title="Change Salesman" @click.stop="startChange('salesperson', item)" />
            <v-list-item prepend-icon="mdi-cash-sync" title="Change Settlement" @click.stop="startChange('settlement', item)" />
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
          <v-autocomplete
            v-if="changeType === 'client'"
            v-model="changeValue"
            v-model:search="clientSearch"
            label="Client"
            :items="clients"
            :item-title="clientOptionTitle"
            item-value="id"
            :loading="optionsLoading"
            no-filter
            placeholder="Search by name or phone"
            variant="outlined"
            @update:search="scheduleClientSearch"
          />
          <v-select
            v-else-if="changeType === 'settlement'"
            v-model="changeValue"
            label="Settlement"
            :items="paymentMethods"
            :item-title="settlementOptionTitle"
            item-value="id"
            :loading="optionsLoading"
            :disabled="optionsLoading"
            variant="outlined"
          />
          <div
            v-if="changeType === 'settlement' && selectedSettlement"
            class="text-body-2 text-medium-emphasis mt-2"
          >
            The settlement will be recorded as
            <strong>{{ settlementAmountLabel(selectedSettlement) }}</strong>
            in {{ selectedSettlement.displayName ?? selectedSettlement.name }}.
          </div>
          <v-select
            v-else
            v-model="changeValue"
            label="Salesman"
            :items="salespersons"
            item-title="fullName"
            item-value="id"
            placeholder="Select a salesman"
            clearable
            :loading="optionsLoading"
            :disabled="optionsLoading"
            variant="outlined"
          /> </v-card-text
        ><v-card-actions
          ><v-spacer /><v-btn variant="text" @click="changeOpen = false">Cancel</v-btn
          ><v-btn
            color="primary"
            :loading="saving"
            :disabled="optionsLoading || (!changeValue && changeType !== 'salesperson')"
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
import { computed, onBeforeUnmount, reactive, ref } from "vue";
import {
  ChangeInvoiceClient,
  ChangeInvoiceSalesperson,
  ChangeInvoiceSettlement,
  DeleteInvoice,
  GetAppSettings,
  GetInvoiceDetails,
  ListInvoicesPage,
  ListPaymentMethods,
  ListSalespersons,
  ReprintInvoice,
  SearchClients,
} from "../../wailsjs/go/main/App";
import { notifyIncomeChanged } from "@/lib/incomeEvents";
import { formatBalance } from "@/lib/format";
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
  optionsLoading = ref(false),
  saving = ref(false),
  printingId = ref<number | null>(null);
const deleteOpen = ref(false),
  deleting = ref(false),
  deleteInvoice = ref<any>(null);
const actionMenus = reactive<Record<number, boolean>>({});
const changeType = ref<"client" | "settlement" | "salesperson">("client"),
  changeValue = ref<number | null>(null),
  changeInvoice = ref<any>(null);
const clients = ref<any[]>([]),
  paymentMethods = ref<any[]>([]),
  salespersons = ref<any[]>([]);
const exchangeRate = ref(90000);
const clientSearch = ref("");
const snackbar = reactive({ show: false, text: "", color: "success" });
const changeTitle = computed(() =>
  changeType.value === "client"
    ? "Change Client"
    : changeType.value === "settlement"
      ? "Change Settlement"
      : "Change Salesman",
);
const selectedSettlement = computed(() =>
  paymentMethods.value.find((method) => method.id === changeValue.value),
);
const settlementCents = computed(() => {
  const payments = changeInvoice.value?.payments;
  if (Array.isArray(payments) && payments.length)
    return payments.reduce((total: number, payment: any) => total + Number(payment.amount || 0), 0);
  return Number(changeInvoice.value?.grandTotal || 0);
});
function money(value: number | undefined) {
  return ((Number(value) || 0) / 100).toFixed(2);
}
function cents(value: number | undefined) {
  return ((Number(value) || 0) / 100).toFixed(2);
}
function settlementOptionTitle(method: any): string {
  return `${method.displayName ?? method.name} (${method.currencyCode})`;
}
function settlementAmountLabel(method: any): string {
  return formatBalance(
    settlementCents.value,
    method.currencyCode ?? "USD",
    exchangeRate.value,
  );
}
function formatDate(value?: string) {
  if (!value) return "Date unavailable";

  // SQLite stores invoice timestamps in UTC without a timezone suffix.
  const normalized = value.includes("T") ? value : value.replace(" ", "T");
  const hasTimezone = /(?:Z|[+-]\d{2}:\d{2})$/i.test(normalized);
  const date = new Date(hasTimezone ? normalized : `${normalized}Z`);
  if (Number.isNaN(date.getTime())) return value.slice(0, 19);

  const pad = (part: number) => String(part).padStart(2, "0");
  return [
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`,
    `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`,
  ].join(" ");
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
  actionMenus[item.id] = false;
  changeType.value = type;
  changeInvoice.value = item;
  changeValue.value =
    type === "client"
      ? item.clientId
      : type === "salesperson"
        ? item.salespersonId || null
        : item.cashboxId;
  changeOpen.value = true;
  if (type === "client") clientSearch.value = "";

  optionsLoading.value = true;
  try {
    const [, invoiceDetails] = await Promise.all([
      ensureEditOptions(type),
      type === "settlement" ? GetInvoiceDetails(item.id) : Promise.resolve(null),
    ]);
    if (invoiceDetails && changeInvoice.value?.id === item.id) {
      changeInvoice.value = invoiceDetails;
      changeValue.value = invoiceDetails.cashboxId;
    }
  } catch (e: any) {
    notify(e?.toString() || "Could not load invoice edit options", "error");
  } finally {
    optionsLoading.value = false;
  }
}
function requestDelete(item: any) {
  actionMenus[item.id] = false;
  deleteInvoice.value = item;
  deleteOpen.value = true;
}
async function deleteSelected() {
  if (!deleteInvoice.value || !auth.isAdmin) return;
  deleting.value = true;
  try {
    await DeleteInvoice(deleteInvoice.value.id);
    notifyIncomeChanged();
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
    else if (changeType.value === "settlement") {
      await ChangeInvoiceSettlement(changeInvoice.value.id, changeValue.value || 0);
      notifyIncomeChanged();
    } else await ChangeInvoiceSalesperson(changeInvoice.value.id, changeValue.value || 0);
    changeOpen.value = false;
    notify("Invoice updated");
    await refresh();
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
type InvoicePageOptions = { page: number; itemsPerPage: number };

let pageActive = true;
let queuedLoad: InvoicePageOptions | null = null;
let activeLoad: Promise<void> | null = null;

function loadInvoices(options: InvoicePageOptions): Promise<void> {
  queuedLoad = { ...options };
  if (!activeLoad) {
    activeLoad = drainInvoiceLoads().finally(() => {
      activeLoad = null;
      if (pageActive && queuedLoad) void loadInvoices(queuedLoad);
    });
  }
  return activeLoad;
}

async function drainInvoiceLoads() {
  loading.value = true;
  try {
    while (pageActive && queuedLoad) {
      const options = queuedLoad;
      queuedLoad = null;
      currentPage.value = options.page;

      try {
        const result = await ListInvoicesPage(options.page, options.itemsPerPage);
        if (!pageActive) return;
        if (queuedLoad) continue;
        items.value = result.items ?? [];
        totalItems.value = result.total ?? 0;
      } catch {
        if (!pageActive) return;
        if (queuedLoad) continue;
        items.value = [];
        totalItems.value = 0;
      }
    }
  } finally {
    if (pageActive) loading.value = false;
  }
}

async function refresh() {
  await loadInvoices({ page: currentPage.value, itemsPerPage: itemsPerPage.value });
}

let paymentMethodOptionsPromise: Promise<void> | null = null;
let salespersonOptionsPromise: Promise<void> | null = null;
let clientSearchTimer: ReturnType<typeof setTimeout> | undefined;
let clientSearchRequest = 0;

function clientOptionTitle(client: any): string {
  return client.phone ? `${client.name} · ${client.phone}` : client.name;
}

async function loadClientOptions(search: string): Promise<void> {
  const request = ++clientSearchRequest;
  optionsLoading.value = true;
  try {
    const loadedClients = await SearchClients(
      search.trim(),
      changeType.value === "client" ? Number(changeValue.value || 0) : 0,
    );
    if (pageActive && request === clientSearchRequest) clients.value = loadedClients;
  } finally {
    if (request === clientSearchRequest) optionsLoading.value = false;
  }
}

function scheduleClientSearch(search: string | null) {
  clearTimeout(clientSearchTimer);
  if (!changeOpen.value || changeType.value !== "client") return;
  clientSearchTimer = setTimeout(() => {
    void loadClientOptions(search ?? "").catch(() => {
      notify("Could not search clients", "error");
    });
  }, 200);
}

function ensureEditOptions(type: "client" | "settlement" | "salesperson"): Promise<void> {
  if (type === "client") {
    return loadClientOptions(clientSearch.value);
  }

  if (type === "settlement") {
    if (!paymentMethodOptionsPromise) {
      paymentMethodOptionsPromise = Promise.all([ListPaymentMethods(), GetAppSettings()])
        .then(([loadedPaymentMethods, settings]) => {
          paymentMethods.value = loadedPaymentMethods;
          const configuredRate = Number(settings?.exchangeRate);
          if (Number.isFinite(configuredRate) && configuredRate > 0)
            exchangeRate.value = configuredRate;
        })
        .catch((error) => {
          paymentMethodOptionsPromise = null;
          throw error;
        });
    }
    return paymentMethodOptionsPromise;
  }

  if (!salespersonOptionsPromise) {
    salespersonOptionsPromise = ListSalespersons()
      .then((loadedSalespersons) => {
        salespersons.value = loadedSalespersons;
      })
      .catch((error) => {
        salespersonOptionsPromise = null;
        throw error;
      });
  }
  return salespersonOptionsPromise;
}

onBeforeUnmount(() => {
  pageActive = false;
  queuedLoad = null;
  clearTimeout(clientSearchTimer);
  clientSearchRequest++;
});
</script>

<style scoped>
.invoices-page {
  box-sizing: border-box;
  height: 100%;
  min-height: 0;
  overflow-y: auto;
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
