<template>
  <div class="invoices-page" @scroll.passive.capture="onTableScroll">
    <v-data-table-virtual
      ref="invoicesTable"
      class="invoices-table"
      :headers="headers"
      :items="items"
      :loading="loading"
      height="100%"
      :item-height="48"
      density="compact"
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
      <template #item.invoiceCode="{ item }">
        <span class="font-weight-bold invoice-code">
          {{ item.invoiceCode || `Invoice #${item.id}` }}
        </span>
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
          {{ item.salespersonName || "Anonymous" }}
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
            <v-list-item prepend-icon="mdi-cash-sync" title="Change Settlement" @click.stop="startChange('settlement', item)" />
            <v-list-item v-if="auth.isAdmin" prepend-icon="mdi-cash-check" title="Fix Rounding" @click.stop="requestRounding(item)" />
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
    </v-data-table-virtual>

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
              selected.payments?.map(settlementAccountName).join(", ") || "No settlement"
            }}</v-chip>
          </div>
          <v-list class="pa-0 mb-4" density="compact">
            <v-list-item v-for="line in selected.items" :key="`${line.barcode}-${line.price}-${line.discount}`">
              <template #title>
                {{ line.name }}
                <v-chip v-if="line.color" size="x-small" variant="tonal" class="ms-2">{{ line.color }}</v-chip>
                <v-chip v-if="line.size" size="x-small" variant="outlined" class="ms-1">{{ line.size }}</v-chip>
              </template
              ><template #subtitle
                >{{ line.qty }} × ${{ Number(line.price).toFixed(2)
                }}<v-icon
                  v-if="line.isPriceOverridden"
                  icon="mdi-star"
                  color="amber-darken-2"
                  size="12"
                  :title="line.priceListName ? `Price from list: ${line.priceListName}` : 'List price'"
                /><span v-if="line.discount"
                  >&nbsp;· {{ line.discount }}% discount<v-icon
                    v-if="line.isDiscountOverridden"
                    icon="mdi-star"
                    color="amber-darken-2"
                    size="12"
                    :title="line.discountListName ? `Discount from list: ${line.discountListName}` : 'List discount'"
                  /></span
                ><span v-if="line.salespersonName"> · Sold by {{ line.salespersonName }}</span></template
              >
              <template v-if="line.note" #append>
                <div class="d-flex align-center ga-3">
                  <v-tooltip :text="line.note" location="top">
                    <template #activator="{ props: tooltipProps }">
                      <v-icon v-bind="tooltipProps" icon="mdi-note-text-outline" size="18" color="primary" />
                    </template>
                  </v-tooltip>
                  <span class="font-weight-bold">${{ Number(line.lineTotal).toFixed(2) }}</span>
                </div>
              </template>
              <template #append
                v-else
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
          <template v-else-if="changeType === 'settlement'">
            <v-card
              v-if="settlementSources.length"
              class="mb-4 pa-3"
              variant="tonal"
              rounded="lg"
            >
              <div class="text-caption font-weight-bold mb-2">Current payments</div>
              <div
                v-for="payment in settlementSources"
                :key="payment.accountId"
                class="settlement-summary-row"
              >
                <span>{{ settlementAccountName(payment) }}</span>
                <strong>{{ settlementSourceAmountLabel(payment) }}</strong>
              </div>
            </v-card>
            <v-select
              :model-value="sourceSettlementId"
              placeholder="Move from"
              :items="settlementSources"
              :item-title="settlementSourceTitle"
              item-value="accountId"
              :loading="optionsLoading"
              :disabled="optionsLoading"
              variant="outlined"
              @update:model-value="setSettlementSource"
            />
            <v-text-field
              v-model.number="settlementAmount"
              class="mt-2"
              :placeholder="`Amount to move (${settlementAmountUnit})`"
              type="number"
              :min="settlementAmountUnit === 'LBP' ? 1 : 0.01"
              :step="settlementAmountUnit === 'LBP' ? 10000 : 0.01"
              :error-messages="settlementAmountError ? [settlementAmountError] : []"
              variant="outlined"
            />
            <v-select
              v-model="changeValue"
              class="mt-2"
              placeholder="Move to"
              :items="destinationPaymentMethods"
              :item-title="settlementOptionTitle"
              item-value="id"
              :loading="optionsLoading"
              :disabled="optionsLoading"
              variant="outlined"
            />
            <div
              v-if="selectedSettlement && transferCents > 0"
              class="text-body-2 text-medium-emphasis mt-2"
            >
              Move <strong>{{ sourceTransferLabel }}</strong> to
              {{ selectedSettlement.displayName ?? selectedSettlement.name }} as
              <strong>{{ settlementAmountLabel(selectedSettlement) }}</strong>.
            </div>
          </template>
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
import {
  ChangeInvoiceClient,
  ChangeInvoiceSalesperson,
  ChangeInvoiceSettlementAmount,
  DeleteInvoice,
  RoundInvoice,
  GetInvoiceDetails,
  ListInvoicesFilteredPage,
  ListPaymentMethods,
  ListSalespersons,
  ReprintInvoice,
  SearchClients,
} from "../../wailsjs/go/main/App";
import { notifyIncomeChanged } from "@/lib/incomeEvents";
import { formatBalance } from "@/lib/format";
import { useCurrencies, findCurrency } from "@/composables/useCurrencies";
import { roundCentsFromCurrencyUnits, roundCurrencyUnitsFromCents } from "@/lib/money";
import { formatLebanesePhone } from "@/lib/phone";
import { useAuthStore } from "../stores/auth";

const PAGE_SIZE = 25;
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
const selected = ref<any>(null),
  detailOpen = ref(false),
  changeOpen = ref(false),
  optionsLoading = ref(false),
  saving = ref(false),
  printingId = ref<number | null>(null);
const deleteOpen = ref(false),
  deleting = ref(false),
  deleteInvoice = ref<any>(null);
const roundingOpen = ref(false),
  rounding = ref(false),
  roundingInvoice = ref<any>(null);
const actionMenus = reactive<Record<number, boolean>>({});
const changeType = ref<"client" | "settlement" | "salesperson">("client"),
  changeValue = ref<number | null>(null),
  changeInvoice = ref<any>(null);
const sourceSettlementId = ref<number | null>(null);
const settlementAmount = ref<number | null>(null);
const clients = ref<any[]>([]),
  paymentMethods = ref<any[]>([]),
  salespersons = ref<any[]>([]);
const clientSearch = ref("");
const snackbar = reactive({ show: false, text: "", color: "success" });
const { currencies, roundingMode } = useCurrencies();
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
const settlementSources = computed(() =>
  Array.isArray(changeInvoice.value?.payments)
    ? changeInvoice.value.payments.filter((payment: any) => Number(payment.amount) > 0)
    : [],
);
const sourceSettlement = computed(() =>
  settlementSources.value.find(
    (payment: any) => Number(payment.accountId) === sourceSettlementId.value,
  ),
);
const sourceSettlementMethod = computed(() =>
  paymentMethods.value.find((method) => method.id === sourceSettlementId.value),
);
const destinationPaymentMethods = computed(() =>
  paymentMethods.value.filter((method) => method.id !== sourceSettlementId.value),
);
const settlementAmountUnit = computed(
  () => sourceSettlementMethod.value?.currencyCode ?? sourceSettlement.value?.currencyCode ?? "USD",
);
const transferCents = computed(() => nativeSettlementAmountToCents(settlementAmount.value));
const settlementAmountError = computed(() => {
  if (changeType.value !== "settlement" || optionsLoading.value) return "";
  if (!sourceSettlement.value) return "Choose an existing settlement";
  if (!Number.isFinite(Number(settlementAmount.value)) || transferCents.value <= 0)
    return "Enter a positive amount";
  if (transferCents.value > Number(sourceSettlement.value.amount))
    return `Maximum: ${settlementSourceAmountLabel(sourceSettlement.value)}`;
  return "";
});
const changeSaveDisabled = computed(() => {
  if (optionsLoading.value) return true;
  if (changeType.value === "salesperson") return false;
  if (changeType.value === "settlement")
    return (
      !sourceSettlementId.value ||
      !changeValue.value ||
      changeValue.value === sourceSettlementId.value ||
      Boolean(settlementAmountError.value)
    );
  return !changeValue.value;
});
const sourceTransferLabel = computed(() =>
  formatBalance(
    transferCents.value,
    settlementAmountUnit.value,
    currencies.value,
    roundingMode.value,
  ),
);
function money(value: number | undefined) {
  return ((Number(value) || 0) / 100).toFixed(2);
}
function cents(value: number | undefined) {
  return ((Number(value) || 0) / 100).toFixed(2);
}
function settlementOptionTitle(method: any): string {
  return `${method.displayName ?? method.name} (${method.currencyCode})`;
}
function settlementAccountName(payment: any): string {
  const name = String(payment.accountName || "Cash");
  const normalizedName = name.trim().toLowerCase();
  if (normalizedName.startsWith("on account")) return "On Account";
  if (normalizedName.startsWith("on coupon")) return "On Coupon";
  const currency = String(payment.currencyCode || "USD").toUpperCase();
  return name.toUpperCase().includes(currency) ? name : `${name} ${currency}`;
}
function settlementSourceTitle(payment: any): string {
  return `${settlementAccountName(payment)} — ${settlementSourceAmountLabel(payment)}`;
}
function settlementSourceAmountLabel(payment: any): string {
  return formatBalance(
    Number(payment.amount || 0),
    payment.currencyCode ?? "USD",
    currencies.value,
    roundingMode.value,
  );
}
function settlementAmountLabel(method: any): string {
  return formatBalance(
    transferCents.value,
    method.currencyCode ?? "USD",
    currencies.value,
    roundingMode.value,
  );
}
function nativeSettlementAmountToCents(value: number | null): number {
  const amount = Number(value);
  if (!Number.isFinite(amount) || amount <= 0) return 0;
  const currency = findCurrency(currencies.value, settlementAmountUnit.value);
  if (currency && currency.cashRoundingUnit > 1)
    return roundCentsFromCurrencyUnits(amount, currency, roundingMode.value);
  return Math.round(amount * 100);
}
function nativeSettlementAmountFromCents(amountCents: number, currencyCode: string): number {
  const currency = findCurrency(currencies.value, currencyCode);
  if (currency && currency.cashRoundingUnit > 1)
    return roundCurrencyUnitsFromCents(amountCents, currency, roundingMode.value);
  return Math.round(amountCents) / 100;
}
function setSettlementSource(accountId: number | null) {
  sourceSettlementId.value = accountId;
  const source = settlementSources.value.find(
    (payment: any) => Number(payment.accountId) === accountId,
  );
  const method = paymentMethods.value.find((candidate) => candidate.id === accountId);
  settlementAmount.value = source
    ? nativeSettlementAmountFromCents(
        Number(source.amount),
        method?.currencyCode ?? source.currencyCode ?? "USD",
      )
    : null;
  if (changeValue.value === accountId)
    changeValue.value = destinationPaymentMethods.value[0]?.id ?? null;
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
function settlementAccountsLabel(accounts?: string[]) {
  return accounts?.length ? accounts.join(", ") : "—";
}
const headers = [
  { title: "Invoice", key: "invoiceCode", width: "145px" },
  { title: "Customer", key: "clientId", width: "145px" },
  { title: "Salesperson", key: "salespersonName", width: "170px" },
  { title: "Paid To", key: "settlementAccounts", sortable: false, width: "145px" },
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
  if (type === "settlement") {
    sourceSettlementId.value = null;
    settlementAmount.value = null;
    changeValue.value = null;
  }

  optionsLoading.value = true;
  try {
    const [, invoiceDetails] = await Promise.all([
      ensureEditOptions(type),
      type === "settlement" ? GetInvoiceDetails(item.id) : Promise.resolve(null),
    ]);
    if (invoiceDetails && changeInvoice.value?.id === item.id) {
      changeInvoice.value = invoiceDetails;
      const source =
        invoiceDetails.payments?.find(
          (payment: any) =>
            Number(payment.accountId) === Number(invoiceDetails.cashboxId) &&
            Number(payment.amount) > 0,
        ) ?? invoiceDetails.payments?.find((payment: any) => Number(payment.amount) > 0);
      setSettlementSource(source?.accountId ?? null);
      changeValue.value = destinationPaymentMethods.value[0]?.id ?? null;
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
    else if (changeType.value === "settlement") {
      await ChangeInvoiceSettlementAmount(
        changeInvoice.value.id,
        sourceSettlementId.value || 0,
        changeValue.value || 0,
        transferCents.value,
      );
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
let pageActive = true;
let nextPage = 1;
let loadGeneration = 0;
let activeLoad: Promise<void> | null = null;
let scrollFrame: number | null = null;

function loadMore(): Promise<void> {
  if (!pageActive || activeLoad || !hasMore.value) return activeLoad ?? Promise.resolve();

  const generation = loadGeneration;
  const page = nextPage;
  loading.value = true;
  activeLoad = (async () => {
    try {
      const result = await ListInvoicesFilteredPage(page, PAGE_SIZE, JSON.stringify(filters));
      if (!pageActive || generation !== loadGeneration) return;

      const pageItems = result.items ?? [];
      const newItems = pageItems.filter((item: any) => {
        if (loadedIds.has(item.id)) return false;
        loadedIds.add(item.id);
        return true;
      });
      items.value = [...items.value, ...newItems];
      totalItems.value = result.total ?? 0;
      initialized.value = true;
      endReached = pageItems.length < PAGE_SIZE;
      nextPage = page + 1;
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
  nextPage = 1;
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

let paymentMethodOptionsPromise: Promise<void> | null = null;
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

function ensureEditOptions(type: "client" | "settlement" | "salesperson"): Promise<void> {
  if (type === "client") {
    return loadClientOptions(clientSearch.value);
  }

  if (type === "settlement") {
    if (!paymentMethodOptionsPromise) {
      paymentMethodOptionsPromise = Promise.all([ListPaymentMethods()])
        .then(([loadedPaymentMethods]) => {
          paymentMethods.value = loadedPaymentMethods;
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

onMounted(() => void refresh());

onBeforeUnmount(() => {
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
.invoices-table :deep(.v-table__wrapper > table) {
  width: 945px !important;
  min-width: 945px;
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
.settlement-summary-row {
  display: flex;
  gap: 12px;
  justify-content: space-between;
  margin-top: 4px;
}
.app-snackbar .v-snackbar__wrapper {
  background: rgba(var(--v-theme-surface-variant), 0.72) !important;
  color: rgb(var(--v-theme-on-surface-variant)) !important;
  border: 1px solid rgba(var(--v-theme-on-surface-variant), 0.1);
  box-shadow: 0 4px 18px rgba(0, 0, 0, 0.12) !important;
}
</style>
