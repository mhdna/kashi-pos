<template>
  <div class="invoices-page" @scroll.passive.capture="onTableScroll">
    <v-data-table-virtual
      ref="invoicesTable"
      class="invoices-table"
      :headers="headers"
      :items="items"
      :loading="loading"
      height="100%"
      :item-height="52"
      fixed-header
      item-value="id"
      hover
      @click:row="openInvoiceRow"
    >
      <template
        v-for="filterHeader in filterableHeaders"
        :key="filterHeader.headerKey"
        #[`header.${filterHeader.headerKey}`]="{ column }"
      >
        <v-menu
          v-model="filterMenus[filterHeader.filterKey]"
          :close-on-content-click="false"
          location="bottom start"
        >
          <template #activator="{ props }">
            <button
              v-bind="props"
              class="invoice-filter-button"
              type="button"
              @click.stop
            >
              <span>{{ column.title }}</span>
              <v-icon
                :icon="filters[filterHeader.filterKey] ? 'mdi-filter' : 'mdi-filter-outline'"
                :color="filters[filterHeader.filterKey] ? 'primary' : undefined"
                size="15"
              />
            </button>
          </template>
          <v-card class="invoice-filter-menu" width="250" @click.stop>
            <v-card-text class="pa-3 pb-2">
              <v-text-field
                v-model="filterDrafts[filterHeader.filterKey]"
                autofocus
                clearable
                density="compact"
                hide-details
                :placeholder="filterHeader.placeholder"
                variant="outlined"
                @keydown.enter="applyFilter(filterHeader.filterKey)"
              />
            </v-card-text>
            <v-card-actions class="px-3 pt-0">
              <v-btn
                size="small"
                variant="text"
                :disabled="!filters[filterHeader.filterKey] && !filterDrafts[filterHeader.filterKey]"
                @click="clearFilter(filterHeader.filterKey)"
              >
                Clear
              </v-btn>
              <v-spacer />
              <v-btn size="small" color="primary" variant="flat" @click="applyFilter(filterHeader.filterKey)">
                Apply
              </v-btn>
            </v-card-actions>
          </v-card>
        </v-menu>
      </template>
      <template #item.syncState="{ item }">
        <v-progress-circular
          v-if="item.syncState === 'pending'"
          indeterminate
          size="12"
          width="2"
          color="warning"
          :title="syncDotTitle(item)"
        />
        <v-icon
          v-else
          icon="mdi-circle"
          size="10"
          :color="syncDotColor(item.syncState)"
          :title="syncDotTitle(item)"
        />
      </template>
      <template #item.invoiceCode="{ item }">
        <span class="font-weight-bold invoice-code">
          {{ item.invoiceCode || `Invoice #${item.id}` }}
        </span>
        <v-chip v-if="item.cancelled" size="x-small" color="error" variant="tonal" class="ml-2">Cancelled</v-chip>
      </template>
      <template #item.clientId="{ item }">
        <span class="invoice-cell-truncate" :title="item.clientName || 'Walk-in customer'">
          {{ item.clientName || "Walk-in customer" }}
        </span>
      </template>
      <template #item.salespersonName="{ item }">
        <v-chip
          size="small"
          variant="tonal"
          prepend-icon="mdi-account-tie"
          :title="item.salespersonName || 'Anonymous'"
        >
          {{ salespersonFirstName(item.salespersonName) }}
        </v-chip>
      </template>
      <template #item.settlementAccounts="{ item }">
        <span
          class="invoice-cell-truncate"
          :class="{ 'text-medium-emphasis': !item.settlementAccounts?.length }"
          :title="settlementAccountsLabel(item.settlementAccounts)"
        >
          {{ settlementAccountsLabel(item.settlementAccounts) }}
        </span>
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
            <v-list-item prepend-icon="mdi-cash-sync" title="Change Settlement" :disabled="settlementLoading" @click.stop="startSettlementChange(item)" />
            <v-list-item v-if="auth.isAdmin" prepend-icon="mdi-cash-check" title="Fix Rounding" @click.stop="requestRounding(item)" />
            <v-list-item v-if="auth.isAdmin && item.isSale && !item.cancelled" prepend-icon="mdi-cancel" title="Cancel Invoice" base-color="error" @click.stop="requestCancel(item)" />
            <v-list-item v-if="auth.isAdmin && !item.cancelled" prepend-icon="mdi-delete-outline" title="Delete Invoice" base-color="error" @click.stop="requestDelete(item)" />
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
    </v-data-table-virtual>

    <v-dialog v-model="detailOpen" max-width="700">
      <v-card v-if="selected">
        <v-card-title class="invoice-detail-header d-flex align-center">
          <div>
            <div class="text-h6">{{ selected.invoiceCode }}</div>
            <div class="text-caption text-medium-emphasis">
              {{ formatDate(selected.createdAt) }}
            </div>
          </div>
          <v-spacer />
          <v-btn icon="mdi-printer-outline" variant="text" :loading="printingId === selected.id" title="Print receipt" @click="reprint(selected)" />
          <v-btn icon="mdi-close" variant="text" aria-label="Close invoice details" @click="detailOpen = false" />
        </v-card-title>
        <v-card-text class="invoice-detail-content">
          <div class="invoice-overview-grid mb-5">
            <div class="invoice-overview-card">
              <v-icon class="invoice-overview-icon" icon="mdi-account-outline" color="grey-darken-1" />
              <div class="invoice-overview-copy">
                <div class="invoice-overview-value">{{ selected.clientName || "Walk-in customer" }}</div>
                <div class="invoice-overview-detail">{{ selected.clientPhone ? formatLebanesePhone(selected.clientPhone) : "No phone number" }}</div>
              </div>
            </div>
            <div class="invoice-overview-card">
              <v-icon class="invoice-overview-icon" icon="mdi-account-tie" color="primary" />
              <div class="invoice-overview-copy">
                <div class="invoice-overview-value">{{ selected.salespersonName || "Anonymous" }}</div>
              </div>
            </div>
            <div class="invoice-overview-card">
              <v-icon class="invoice-overview-icon" icon="mdi-star-four-points-outline" color="amber-darken-2" />
              <div class="invoice-overview-copy">
                <div class="invoice-overview-value">{{ Number(selected.loyaltyPointsEarned ?? Math.floor(Number(selected.grandTotal || 0) / 100)).toLocaleString() }} pts</div>
              </div>
            </div>
            <div class="invoice-overview-card">
              <v-icon class="invoice-overview-icon" :icon="selected.notes?.trim() ? 'mdi-note-text-outline' : 'mdi-note-off-outline'" color="secondary" />
              <div class="invoice-overview-copy">
                <div class="invoice-overview-value" :class="{ 'text-medium-emphasis': !selected.notes?.trim() }">{{ selected.notes?.trim() || "None" }}</div>
              </div>
            </div>
          </div>

          <section class="invoice-detail-section mb-4">
            <div class="section-label">Settlement</div>
            <div v-if="selected.payments?.length" class="settlement-list">
              <div v-for="payment in selected.payments" :key="`${payment.accountId}-${payment.amount}`" class="settlement-row">
                <div class="d-flex align-center ga-2"><v-icon icon="mdi-credit-card-outline" size="19" color="primary" /><span>{{ settlementAccountName(payment) }}</span></div>
                <span class="font-weight-bold">${{ cents(payment.amount) }}</span>
              </div>
              <div class="settlement-row settlement-total"><span>Total paid</span><span>${{ cents(selectedPaid) }}</span></div>
            </div>
            <div v-else class="text-body-2 text-medium-emphasis">No settlement recorded.</div>
          </section>

          <div class="invoice-detail-scroll">
            <div class="section-label mb-2">Items</div>
            <v-list class="invoice-items-list pa-0 mb-4" density="compact">
              <v-list-item v-for="line in selected.items" :key="`${line.barcode}-${line.price}-${line.discount}`">
                <template #title>
                  {{ line.name }}
                  <span v-if="line.color || line.size" class="text-caption text-medium-emphasis ms-2">{{ [line.color, line.size].filter(Boolean).join(" · ") }}</span>
                </template>
                <template #subtitle>{{ line.qty }} × ${{ Number(line.price).toFixed(2) }}<span v-if="line.discount"> · {{ line.discount }}% discount</span><span v-if="line.salespersonName"> · Sold by {{ line.salespersonName }}</span></template>
                <template #append>
                  <v-tooltip v-if="line.note" :text="line.note" location="top"><template #activator="{ props: tooltipProps }"><v-icon v-bind="tooltipProps" icon="mdi-note-text-outline" size="18" color="primary" class="me-3" /></template></v-tooltip>
                  <span class="font-weight-bold">${{ Number(line.lineTotal).toFixed(2) }}</span>
                </template>
              </v-list-item>
            </v-list>
            <div class="totals invoice-totals">
              <div><span>Subtotal</span><span>${{ cents(selected.subtotal) }}</span></div>
              <div><span>Discount</span><span class="text-red">-${{ cents(Number(selected.subtotal) - Number(selected.discountedTotal)) }}</span></div>
              <v-divider class="my-2" />
              <div class="text-body-1 font-weight-bold"><span>Total</span><span>${{ cents(selected.grandTotal) }}</span></div>
            </div>
          </div>
        </v-card-text>
      </v-card>
    </v-dialog>

    <v-dialog v-model="cancelOpen" max-width="460">
      <v-card>
        <v-card-title>Cancel invoice?</v-card-title>
        <v-card-text>
          <p class="mb-3">
            {{ cancelTarget?.invoiceCode || "This invoice" }} is returned in full: the
            ${{ money(cancelTarget?.grandTotal) }} is refunded to its payment account and the
            items come back. Both stay on record marked Cancelled, and neither counts in sales
            stats, here or in kashi.
          </p>
          <v-text-field v-model="cancelReason" label="Reason (optional)" density="compact" hide-details />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="cancelOpen = false">Keep</v-btn>
          <v-btn color="error" :loading="cancelling" @click="cancelSelected">Cancel invoice</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="deleteOpen" max-width="420">
      <v-card
        ><v-card-title>Delete invoice?</v-card-title
        ><v-card-text
          >This permanently deletes {{ deleteInvoice?.invoiceCode || "this invoice" }} and its
          payment records. Only an invoice that hasn't been uploaded to kashi can be deleted;
          for a sale that has, use Cancel Invoice.</v-card-text
        ><v-card-actions
          ><v-spacer /><v-btn variant="text" @click="deleteOpen = false">Cancel</v-btn
          ><v-btn color="error" :loading="deleting" @click="deleteSelected"
            >Delete</v-btn
          ></v-card-actions
        ></v-card
      >
    </v-dialog>
    <v-dialog v-model="roundingOpen" max-width="420">
      <v-card>
        <v-card-title>Fix invoice rounding?</v-card-title>
        <v-card-text>
          This applies the current rounding settings to
          {{ roundingInvoice?.invoiceCode || "this invoice" }} and updates its payments,
          balances, income, and receipt total.
        </v-card-text>
        <v-card-actions>
          <v-spacer /><v-btn variant="text" @click="roundingOpen = false">Cancel</v-btn>
          <v-btn color="primary" :loading="rounding" @click="roundSelected">Fix rounding</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
    <v-dialog v-model="changeOpen" max-width="420">
      <v-card
        ><v-card-title>{{ changeTitle }}</v-card-title
        ><v-card-text>
          <v-autocomplete
            v-if="changeType === 'client'"
            :model-value="changeValue"
            v-model:search="clientSearch"
            :items="clients"
            :item-title="clientOptionTitle"
            item-value="id"
            :loading="optionsLoading"
            no-filter
            placeholder="Search by name or phone"
            variant="outlined"
            @update:model-value="changeValue = $event"
            @update:search="scheduleClientSearch"
          />
          <v-select
            v-else
            v-model="changeValue"
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
            :disabled="changeSaveDisabled"
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
import { computed, onBeforeUnmount, onMounted, reactive, ref, shallowRef } from "vue";
import { useRouter } from "vue-router";
import {
  ChangeInvoiceClient,
  ChangeInvoiceSalesperson,
  CancelInvoice,
  DeleteInvoice,
  RoundInvoice,
  GetInvoiceDetails,
  GetInvoiceForSettlementChange,
  GetInvoiceSyncStates,
  ListInvoicesFilteredPage,
  ListSalespersons,
  ReprintInvoice,
  SearchClients,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { notifyIncomeChanged } from "@/lib/incomeEvents";
import { pendingSettlementInvoice } from "@/composables/useNavActions";
import { toClient } from "@/lib/client";
import { formatLebanesePhone } from "@/lib/phone";
import { formatServerDateTime } from "@/lib/datetime";
import { useAuthStore } from "../stores/auth";

const PAGE_SIZE = 20;
const PRELOAD_DISTANCE_PX = 600;
type InvoiceFilterKey = "invoiceCode" | "customer" | "salesperson" | "paidTo" | "date" | "total";
type InvoiceFilterValues = Record<InvoiceFilterKey, string>;

const emptyInvoiceFilters = (): InvoiceFilterValues => ({
  invoiceCode: "",
  customer: "",
  salesperson: "",
  paidTo: "",
  date: "",
  total: "",
});
const filters = reactive<InvoiceFilterValues>(emptyInvoiceFilters());
const filterDrafts = reactive<InvoiceFilterValues>(emptyInvoiceFilters());
const filterMenus = reactive<Record<InvoiceFilterKey, boolean>>({
  invoiceCode: false,
  customer: false,
  salesperson: false,
  paidTo: false,
  date: false,
  total: false,
});
const filterableHeaders: {
  headerKey: string;
  filterKey: InvoiceFilterKey;
  placeholder: string;
}[] = [
  { headerKey: "invoiceCode", filterKey: "invoiceCode", placeholder: "Invoice code contains…" },
  { headerKey: "clientId", filterKey: "customer", placeholder: "Customer contains…" },
  { headerKey: "salespersonName", filterKey: "salesperson", placeholder: "Salesperson contains…" },
  { headerKey: "settlementAccounts", filterKey: "paidTo", placeholder: "Payment account contains…" },
  { headerKey: "createdAt", filterKey: "date", placeholder: "Date, e.g. 2026-08-21" },
  { headerKey: "grandTotal", filterKey: "total", placeholder: "Amount, e.g. 25.00" },
];
let endReached = false;
const loadedIds = new Set<number>();
const items = shallowRef<any[]>([]),
  totalItems = ref(0),
  loading = ref(false),
  initialized = ref(false);
const hasMore = computed(
  () => !initialized.value || (!endReached && items.value.length < totalItems.value),
);
const invoicesTable = ref<{ scrollToIndex: (index: number) => void } | null>(null);
const auth = useAuthStore();
const router = useRouter();
const selected = ref<any>(null),
  detailOpen = ref(false),
  changeOpen = ref(false),
  optionsLoading = ref(false),
  saving = ref(false),
  printingId = ref<number | null>(null);
const deleteOpen = ref(false),
  deleting = ref(false),
  deleteInvoice = ref<any>(null);
const cancelOpen = ref(false),
  cancelling = ref(false),
  cancelTarget = ref<any>(null),
  cancelReason = ref("");
const roundingOpen = ref(false),
  rounding = ref(false),
  roundingInvoice = ref<any>(null);
const actionMenus = reactive<Record<number, boolean>>({});
const changeType = ref<"client" | "salesperson">("client"),
  changeValue = ref<number | null>(null),
  changeInvoice = ref<any>(null);
const clients = ref<any[]>([]),
  salespersons = ref<any[]>([]);
const clientSearch = ref("");
const snackbar = reactive({ show: false, text: "", color: "success" });
const settlementLoading = ref(false);
const changeTitle = computed(() => (changeType.value === "client" ? "Change Client" : "Change Salesman"));
const selectedPaid = computed(() =>
  (selected.value?.payments ?? []).reduce((sum: number, payment: any) => sum + Number(payment.amount || 0), 0),
);
const changeSaveDisabled = computed(() => {
  if (optionsLoading.value) return true;
  if (changeType.value === "salesperson") return false;
  return !changeValue.value;
});
function money(value: number | undefined) {
  return ((Number(value) || 0) / 100).toFixed(2);
}
function cents(value: number | undefined) {
  return ((Number(value) || 0) / 100).toFixed(2);
}
function settlementAccountName(payment: any): string {
  const name = String(payment.accountName || "Cash");
  const normalizedName = name.trim().toLowerCase();
  if (normalizedName.startsWith("on account")) return "On Account";
  if (normalizedName.startsWith("on coupon")) return "On Coupon";
  return name;
}
function formatDate(value?: string) {
  return value ? formatServerDateTime(value) : "Date unavailable";
}
function settlementAccountsLabel(accounts?: string[]) {
  return accounts?.length ? accounts.join(", ") : "—";
}
function salespersonFirstName(name?: string) {
  const trimmed = name?.trim();
  return trimmed ? trimmed.split(/\s+/)[0] : "Anonymous";
}
// The first column is each invoice's sync state with kashi, shown to every
// user so cashiers can see their sales upload.
const headers = [
  { title: "", key: "syncState", sortable: false, width: "28px" },
  { title: "Invoice", key: "invoiceCode", width: "165px" },
  { title: "Customer", key: "clientId", width: "120px" },
  { title: "Salesperson", key: "salespersonName", width: "120px" },
  { title: "Paid To", key: "settlementAccounts", sortable: false, width: "120px" },
  { title: "Date", key: "createdAt", width: "170px" },
  { title: "Total", key: "grandTotal", width: "80px", align: "end" as const },
  {
    title: "",
    key: "actions",
    sortable: false,
    width: "90px",
    align: "end" as const,
  },
];
function syncDotColor(state?: string) {
  if (state === "synced") return "success";
  if (state === "failed") return "error";
  if (state === "pending") return "warning";
  return "grey-lighten-1";
}
function syncDotTitle(item: { syncState?: string; syncError?: string }) {
  const reason = item.syncError ? ` (${item.syncError})` : "";
  if (item.syncState === "synced") return "Confirmed on kashi";
  if (item.syncState === "failed") return `Rejected by kashi${reason}`;
  if (item.syncState === "pending") return item.syncError ? `Uploading — retrying${reason}` : "Uploading to kashi…";
  return "Not synced";
}

// Live sync column: whenever the outbox moves (an upload lands, fails, or
// is retried), re-read just the invoices still showing a spinner and swap
// in their new state -- not a full refresh(), which would reset the scroll
// position and the loaded pages. sync_state_changed only fires on a real
// transition, so this costs nothing while the till is idle or offline.
let syncStatesInFlight = false;
let syncStatesAgain = false;
async function refreshSyncStates() {
  if (syncStatesInFlight) {
    syncStatesAgain = true;
    return;
  }
  const pendingIds = items.value.filter((item) => item.syncState === "pending").map((item) => item.id);
  if (pendingIds.length === 0) return;
  syncStatesInFlight = true;
  try {
    const states = await GetInvoiceSyncStates(pendingIds);
    if (!pageActive) return;
    const byId = new Map(states.map((s) => [s.invoiceId, s]));
    items.value = items.value.map((item) => {
      const next = byId.get(item.id);
      if (!next || (next.state === item.syncState && next.error === item.syncError)) return item;
      return { ...item, syncState: next.state, syncError: next.error };
    });
  } catch {
    // Best-effort: the next sync_state_changed tries again.
  } finally {
    syncStatesInFlight = false;
    if (syncStatesAgain) {
      syncStatesAgain = false;
      void refreshSyncStates();
    }
  }
}
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
async function startChange(type: "client" | "salesperson", item: any) {
  actionMenus[item.id] = false;
  changeType.value = type;
  changeInvoice.value = item;
  changeValue.value = type === "client" ? item.clientId : item.salespersonId || null;
  changeOpen.value = true;
  if (type === "client") clientSearch.value = "";

  optionsLoading.value = true;
  try {
    await ensureEditOptions(type);
  } catch (e: any) {
    notify(e?.toString() || "Could not load invoice edit options", "error");
  } finally {
    optionsLoading.value = false;
  }
}

async function startSettlementChange(item: any) {
  actionMenus[item.id] = false;
  settlementLoading.value = true;
  try {
    const details = await GetInvoiceForSettlementChange(item.id);
    pendingSettlementInvoice.value = {
      id: details.id,
      invoiceCode: details.invoiceCode,
      items: details.items ?? [],
      client: toClient({ id: details.clientId, name: details.clientName, phone: details.clientPhone }),
      salesperson:
        details.salespersonId > 0
          ? { id: details.salespersonId, fullName: details.salespersonName }
          : null,
      notes: details.notes ?? "",
      grandTotalCents: Number(details.grandTotal ?? 0),
      entries: details.entries ?? [],
    };
    await router.push("/home");
  } catch (e: any) {
    notify(e?.toString() || "Could not load invoice for settlement change", "error");
  } finally {
    settlementLoading.value = false;
  }
}
function requestCancel(item: any) {
  actionMenus[item.id] = false;
  cancelTarget.value = item;
  cancelReason.value = "";
  cancelOpen.value = true;
}
async function cancelSelected() {
  if (!cancelTarget.value || !auth.isAdmin) return;
  cancelling.value = true;
  try {
    await CancelInvoice(cancelTarget.value.id, cancelReason.value);
    notifyIncomeChanged();
    cancelOpen.value = false;
    notify("Invoice cancelled");
    await refresh();
  } catch (e: any) {
    notify(e?.toString() || "Could not cancel invoice", "error");
  } finally {
    cancelling.value = false;
  }
}
function requestDelete(item: any) {
  actionMenus[item.id] = false;
  deleteInvoice.value = item;
  deleteOpen.value = true;
}
function requestRounding(item: any) {
  actionMenus[item.id] = false;
  roundingInvoice.value = item;
  roundingOpen.value = true;
}
async function roundSelected() {
  if (!roundingInvoice.value || !auth.isAdmin) return;
  rounding.value = true;
  try {
    await RoundInvoice(roundingInvoice.value.id);
    notifyIncomeChanged();
    roundingOpen.value = false;
    notify("Invoice rounding fixed");
    await refresh();
  } catch (e: any) {
    notify(e?.toString() || "Could not fix invoice rounding", "error");
  } finally {
    rounding.value = false;
  }
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
    else await ChangeInvoiceSalesperson(changeInvoice.value.id, changeValue.value || 0);
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
let pageActive = true;
// Keyset cursor: id of the last invoice we've loaded. 0 asks for the first
// page. Each fetch is an indexed `id < cursor` seek, so scrolling deep
// stays as cheap as the first page (no OFFSET re-scan).
let nextCursor = 0;
let loadGeneration = 0;
let activeLoad: Promise<void> | null = null;
let scrollFrame: number | null = null;

function loadMore(): Promise<void> {
  if (!pageActive || activeLoad || !hasMore.value) return activeLoad ?? Promise.resolve();

  const generation = loadGeneration;
  const cursor = nextCursor;
  loading.value = true;
  activeLoad = (async () => {
    try {
      const result = await ListInvoicesFilteredPage(cursor, PAGE_SIZE, JSON.stringify(filters));
      if (!pageActive || generation !== loadGeneration) return;

      const pageItems = result.items ?? [];
      const newItems = pageItems.filter((item: any) => {
        if (loadedIds.has(item.id)) return false;
        loadedIds.add(item.id);
        return true;
      });
      items.value = [...items.value, ...newItems];
      // The backend only counts on the first page (cursor 0) and returns
      // total = -1 ("unchanged") thereafter, so infinite scroll never
      // re-runs COUNT(*) over the whole filtered set.
      if (typeof result.total === "number" && result.total >= 0) totalItems.value = result.total;
      initialized.value = true;
      endReached = pageItems.length < PAGE_SIZE;
      if (pageItems.length > 0) nextCursor = pageItems[pageItems.length - 1].id;
    } catch (error: any) {
      if (!pageActive || generation !== loadGeneration) return;
      initialized.value = true;
      notify(error?.toString() || "Could not load invoices", "error");
    }
  })().finally(() => {
    if (generation === loadGeneration) loading.value = false;
    activeLoad = null;
  });
  return activeLoad;
}

function onTableScroll(event: Event) {
  if (scrollFrame !== null || loading.value || !hasMore.value) return;
  const target = event.target;
  if (!(target instanceof HTMLElement) || !target.classList.contains("v-table__wrapper")) return;

  scrollFrame = requestAnimationFrame(() => {
    scrollFrame = null;
    const remaining = target.scrollHeight - target.scrollTop - target.clientHeight;
    if (remaining <= PRELOAD_DISTANCE_PX) void loadMore();
  });
}

async function refresh() {
  const previousLoad = activeLoad;
  loadGeneration++;
  items.value = [];
  totalItems.value = 0;
  initialized.value = false;
  endReached = false;
  loadedIds.clear();
  nextCursor = 0;
  invoicesTable.value?.scrollToIndex(0);
  if (previousLoad) await previousLoad;
  if (pageActive) await loadMore();
}

function applyFilter(key: InvoiceFilterKey) {
  filters[key] = filterDrafts[key].trim();
  filterMenus[key] = false;
  void refresh();
}

function clearFilter(key: InvoiceFilterKey) {
  const wasActive = Boolean(filters[key]);
  filterDrafts[key] = "";
  filters[key] = "";
  filterMenus[key] = false;
  if (wasActive) void refresh();
}

let salespersonOptionsPromise: Promise<void> | null = null;
let clientSearchTimer: ReturnType<typeof setTimeout> | undefined;
let clientSearchRequest = 0;

function clientOptionTitle(client: any): string {
  return client.phone ? `${client.name} · ${formatLebanesePhone(client.phone)}` : client.name;
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

function ensureEditOptions(type: "client" | "salesperson"): Promise<void> {
  if (type === "client") {
    return loadClientOptions(clientSearch.value);
  }

  if (!salespersonOptionsPromise) {
    salespersonOptionsPromise = ListSalespersons()
      .then((loadedSalespersons) => {
        // Staff who only punch attendance can't be set as a sale's salesperson.
        salespersons.value = loadedSalespersons.filter((s: any) => s.isSalesperson !== false && s.isCurrent !== false);
      })
      .catch((error) => {
        salespersonOptionsPromise = null;
        throw error;
      });
  }
  return salespersonOptionsPromise;
}

let stopSyncStateListener: (() => void) | null = null;

onMounted(() => {
  void refresh();
  stopSyncStateListener = EventsOn("sync_state_changed", () => void refreshSyncStates());
});

onBeforeUnmount(() => {
  stopSyncStateListener?.();
  pageActive = false;
  loadGeneration++;
  if (scrollFrame !== null) cancelAnimationFrame(scrollFrame);
  clearTimeout(clientSearchTimer);
  clientSearchRequest++;
});
</script>

<style scoped>
.invoices-page {
  box-sizing: border-box;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}
.invoices-table {
  height: 100%;
}
.invoices-table :deep(.v-table__wrapper) {
  overflow-x: hidden;
}
.invoices-table :deep(.v-table__wrapper > table) {
  width: 100% !important;
  table-layout: fixed;
}
.invoices-table :deep(th),
.invoices-table :deep(td) {
  padding-inline: 8px !important;
}
.invoice-filter-button {
  display: inline-flex;
  width: 100%;
  height: 100%;
  align-items: center;
  padding: 0;
  border: 0;
  background: none;
  gap: 4px;
  color: inherit;
  cursor: pointer;
  font: inherit;
  text-align: inherit;
}
.invoice-filter-menu {
  overflow: visible;
}
.invoice-cell-truncate {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.invoice-code {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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
.invoice-detail-header {
  border-bottom: 1px solid rgba(var(--v-border-color), 0.08);
}
.invoice-detail-content {
  display: flex;
  flex-direction: column;
  max-height: min(72vh, 760px);
  min-height: 0;
  overflow: hidden;
}
.invoice-detail-scroll {
  min-height: 0;
  overflow-y: auto;
  padding-inline: 2px;
}
.invoice-overview-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}
.invoice-overview-card {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  height: 50px;
  padding: 12px;
  border: 1px solid rgba(var(--v-border-color), 0.12);
  border-radius: 12px;
  background: rgba(var(--v-theme-on-surface), 0.025);
}
.invoice-overview-icon {
  flex: 0 0 auto;
  font-size: 30px !important;
}
.invoice-overview-copy { min-width: 0; }
.invoice-overview-label,
.section-label {
  margin-bottom: 8px;
  color: rgb(var(--v-theme-on-surface));
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}
.invoice-overview-value,
.invoice-overview-detail {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.invoice-overview-value {
  margin-bottom: 5px;
  font-size: 0.92rem;
  font-weight: 700;
}
.invoice-overview-detail {
  color: rgba(var(--v-theme-on-surface), 0.62);
  font-size: 0.72rem;
}
.settlement-list { border: 1px solid rgba(var(--v-border-color), 0.12); border-radius: 10px; overflow: hidden; }
.settlement-row { display: flex; justify-content: space-between; align-items: center; gap: 16px; padding: 10px 13px; }
.settlement-row + .settlement-row { border-top: 1px solid rgba(var(--v-border-color), 0.08); }
.settlement-total { font-weight: 700; background: rgba(var(--v-theme-on-surface), 0.035); }
.invoice-items-list { border-top: 1px solid rgba(var(--v-border-color), 0.1); border-bottom: 1px solid rgba(var(--v-border-color), 0.1); }
.invoice-totals { max-width: 360px; margin-left: auto; }
@media (max-width: 560px) {
  .invoice-overview-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
</style>
