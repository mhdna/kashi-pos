<template>
  <div class="clients-page pa-4">
    <v-card class="client-list-card">
      <div :key="listKey" class="client-list" @scroll.passive="handleScroll">
        <v-list density="compact">
          <v-list-item
            v-for="client in clients"
            :key="client.id"
            class="client-list-item"
            link
            @click="openEdit(client)"
          >
            <template #prepend>
              <InitialsAvatar
                :name="client.name ?? ''"
                :color="colors[client.id % colors.length] ?? 'grey'"
              />
            </template>
            <template #title>
              <div class="client-primary-row">
                <span class="client-name font-weight-medium text-truncate">{{ client.name }}</span>
                <span class="client-phone text-medium-emphasis">{{
                  formatLebanesePhone(client.phone)
                }}</span>
                <span class="client-points text-medium-emphasis">
                  {{ formatLoyaltyPoints(client.validLoyaltyPoints) }} pts
                </span>
                <v-chip
                  :color="loyaltyTierColor(loyaltyTier(Number(client.totalLoyaltyPoints ?? 0)))"
                  size="x-small"
                  variant="tonal"
                >
                  {{ loyaltyTier(Number(client.totalLoyaltyPoints ?? 0)) }}
                </v-chip>
                <v-chip
                  v-if="Number(client.accountBalance) > 0"
                  color="orange-darken-2"
                  size="x-small"
                  variant="tonal"
                >
                  {{ formatUsd(Number(client.accountBalance) / 100) }}
                </v-chip>
              </div>
            </template>
            <template #append>
              <v-btn
                icon="mdi-wallet"
                variant="text"
                density="comfortable"
                title="View on-account balance"
                @click.stop="openAccount(client)"
              />
            </template>
          </v-list-item>
        </v-list>
        <div v-if="loading" class="client-list-status">
          <v-progress-circular indeterminate color="primary" size="24" width="2" />
        </div>
        <div v-else-if="loadError" class="client-list-status">
          <v-btn size="small" variant="text" @click="loadMore">Retry</v-btn>
        </div>
        <div v-else-if="!hasMore" class="client-list-status text-medium-emphasis">
          {{ clients.length ? "All clients loaded" : "No clients found" }}
        </div>
      </div>
      <v-divider />
      <div class="client-search-row">
        <v-text-field
          v-model="search"
          class="client-search-control"
          density="compact"
          hide-details
          placeholder="Search clients..."
          variant="outlined"
          autofocus
        />
      </div>
      <v-card-text class="client-inline-keyboard">
        <InlineKeyboard v-model="search" :max-length="80" layout="code" />
      </v-card-text>
    </v-card>
  </div>

  <v-dialog v-model="accountOpen" max-width="820">
    <v-card>
      <v-card-title class="d-flex align-center">
        {{ accountDetails?.client?.name || accountTarget?.name }} — On Account
        <v-spacer />
        <v-chip color="orange-darken-2" variant="tonal">
          Balance: {{ formatUsd(Number(accountDetails?.balance || 0) / 100) }}
        </v-chip>
      </v-card-title>
      <v-progress-linear v-if="accountLoading" indeterminate />
      <v-card-text v-else>
        <div class="text-subtitle-2 mb-2">Invoices</div>
        <v-table density="compact">
          <thead>
            <tr>
              <th>Invoice</th>
              <th>Date</th>
              <th class="text-end">Charged</th>
              <th class="text-end">Paid</th>
              <th class="text-end">Due</th>
              <th />
            </tr>
          </thead>
          <tbody>
            <tr v-for="entry in accountDetails?.entries || []" :key="entry.id">
              <td>{{ entry.invoiceCode }}</td>
              <td>{{ entry.createdAt }}</td>
              <td class="text-end">{{ formatUsd(Number(entry.amount) / 100) }}</td>
              <td class="text-end">{{ formatUsd(Number(entry.paid) / 100) }}</td>
              <td class="text-end font-weight-bold">
                {{ formatUsd(Number(entry.outstanding) / 100) }}
              </td>
              <td class="text-end">
                <v-btn
                  v-if="Number(entry.outstanding) > 0"
                  size="small"
                  color="primary"
                  variant="tonal"
                  @click="startPayment(entry)"
                  >Pay</v-btn
                >
                <v-chip v-else size="x-small" color="green" variant="tonal">Paid</v-chip>
              </td>
            </tr>
            <tr v-if="!accountDetails?.entries?.length">
              <td colspan="6" class="text-center text-medium-emphasis py-4">
                No on-account invoices.
              </td>
            </tr>
          </tbody>
        </v-table>

        <v-expand-transition>
          <v-sheet v-if="payTarget" class="pa-4 mt-4" rounded border>
            <div class="text-subtitle-2 mb-3">Pay {{ payTarget.invoiceCode }}</div>
            <div class="d-flex ga-3 align-start">
              <v-select
                v-model="paymentMethodId"
                :items="collectableMethods"
                item-title="displayName"
                item-value="id"
                placeholder="Payment method"
                variant="outlined"
                density="compact"
                style="flex: 1"
              />
              <v-text-field
                v-model.number="paymentAmount"
                :placeholder="`Amount (${paymentCurrency})`"
                type="number"
                min="0"
                :step="paymentCurrency === 'LBP' ? 1000 : 0.01"
                variant="outlined"
                density="compact"
                :error-messages="paymentError ? [paymentError] : []"
                style="flex: 1"
              />
              <v-btn variant="text" @click="payTarget = null">Cancel</v-btn>
              <v-btn
                color="primary"
                :loading="paying"
                :disabled="Boolean(paymentError) || paymentCents <= 0"
                @click="submitPayment"
                >Save payment</v-btn
              >
            </div>
          </v-sheet>
        </v-expand-transition>

        <template v-if="accountDetails?.payments?.length">
          <div class="text-subtitle-2 mt-5 mb-2">Payment history</div>
          <v-list density="compact" border rounded>
            <v-list-item v-for="payment in accountDetails.payments" :key="payment.id">
              <template #title
                >{{ payment.accountName }} · {{ formatUsd(Number(payment.amount) / 100) }}</template
              >
              <template #subtitle>{{ payment.createdAt }} · Shift #{{ payment.shiftId }}</template>
            </v-list-item>
          </v-list>
        </template>
      </v-card-text>
      <v-card-actions
        ><v-spacer /><v-btn variant="text" @click="accountOpen = false"
          >Close</v-btn
        ></v-card-actions
      >
    </v-card>
  </v-dialog>

  <v-dialog v-model="editOpen" max-width="420">
    <v-card>
      <v-card-title>Edit client</v-card-title>
      <v-card-text>
        <v-text-field v-model="editForm.name" placeholder="Name" variant="outlined" autofocus />
        <v-text-field v-model="editForm.phone" placeholder="Phone" variant="outlined" />
      </v-card-text>
      <v-card-actions>
        <v-btn v-if="auth.isAdmin" color="error" variant="text" @click="openDeleteFromEdit"
          >Delete</v-btn
        >
        <v-spacer />
        <v-btn variant="text" @click="editOpen = false">Cancel</v-btn>
        <v-btn
          color="primary"
          :loading="saving"
          :disabled="!editForm.name.trim() || !editForm.phone.trim()"
          @click="saveEdit"
          >Save</v-btn
        >
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="deleteOpen" max-width="420">
    <v-card>
      <v-card-title>Delete client?</v-card-title>
      <v-card-text
        >This will delete {{ deleteTarget?.name || "this client" }}. Clients linked to invoices
        cannot be deleted.</v-card-text
      >
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="deleteOpen = false">Cancel</v-btn>
        <v-btn color="error" :loading="deleting" @click="deleteSelected">Delete</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-snackbar v-model="snackbar.show" timeout="3500">{{ snackbar.text }}</v-snackbar>
</template>

<script setup lang="ts">
import { computed, ref, onBeforeUnmount, onMounted, watch, reactive } from "vue";
import {
  DeleteClient,
  GetAppSettings,
  GetClientAccountDetails,
  ListClientsCursor,
  ListPaymentMethods,
  PayClientAccount,
  UpdateClient,
} from "../../wailsjs/go/main/App";
import InitialsAvatar from "@/components/InitialsAvatar.vue";
import InlineKeyboard from "@/components/InlineKeyboard.vue";
import { useAuthStore } from "@/stores/auth";
import { formatLoyaltyPoints, loyaltyTier, loyaltyTierColor } from "@/lib/client";
import { formatUsd } from "@/lib/format";
import { formatLebanesePhone } from "@/lib/phone";
import { notifyIncomeChanged } from "@/lib/incomeEvents";

const colors = ["grey-darken-1"];
const auth = useAuthStore();

type ClientItem = {
  id: number;
  name: string;
  phone: string;
  totalLoyaltyPoints: number;
  validLoyaltyPoints: number;
  createdAt: string;
  accountBalance: number;
};

const batchSize = 20;
const clients = ref<ClientItem[]>([]);
const search = ref("");
const debouncedSearch = ref("");
const cursor = ref(0);
const listKey = ref(0);
const loading = ref(false);
const hasMore = ref(true);
const loadError = ref(false);
const editOpen = ref(false);
const deleteOpen = ref(false);
const saving = ref(false);
const deleting = ref(false);
const editTarget = ref<any>(null);
const deleteTarget = ref<any>(null);
const editForm = reactive({ name: "", phone: "" });
const snackbar = reactive({ show: false, text: "" });
const accountOpen = ref(false);
const accountLoading = ref(false);
const accountTarget = ref<any>(null);
const accountDetails = ref<any>(null);
const paymentMethods = ref<any[]>([]);
const exchangeRate = ref(90000);
const payTarget = ref<any>(null);
const paymentMethodId = ref<number | null>(null);
const paymentAmount = ref<number | null>(null);
const paying = ref(false);
let requestSequence = 0;
let searchTimer: ReturnType<typeof setTimeout> | undefined;

const collectableMethods = computed(() =>
  paymentMethods.value.filter(
    (method) => !["on account", "on coupon"].includes(String(method.name).toLowerCase()),
  ),
);
const selectedPaymentMethod = computed(() =>
  collectableMethods.value.find((method) => method.id === paymentMethodId.value),
);
const paymentCurrency = computed(() => selectedPaymentMethod.value?.currencyCode || "USD");
const paymentCents = computed(() => {
  const amount = Number(paymentAmount.value);
  if (!Number.isFinite(amount) || amount <= 0) return 0;
  return paymentCurrency.value === "LBP"
    ? Math.round((amount * 100) / exchangeRate.value)
    : Math.round(amount * 100);
});
const paymentError = computed(() => {
  if (!payTarget.value || !paymentMethodId.value) return "Choose a payment method";
  if (paymentCents.value <= 0) return "Enter a positive amount";
  if (paymentCents.value > Number(payTarget.value.outstanding))
    return "Payment exceeds the amount due";
  return "";
});

async function openAccount(item: any) {
  accountTarget.value = item;
  accountOpen.value = true;
  payTarget.value = null;
  accountLoading.value = true;
  try {
    const [details, methods, settings] = await Promise.all([
      GetClientAccountDetails(item.id),
      ListPaymentMethods(),
      GetAppSettings(),
    ]);
    accountDetails.value = details;
    paymentMethods.value = (methods as any[]) || [];
    const rate = Number((settings as any)?.exchangeRate);
    if (Number.isFinite(rate) && rate > 0) exchangeRate.value = rate;
  } catch (e: any) {
    snackbar.text = e?.toString() || "Could not load client account";
    snackbar.show = true;
  } finally {
    accountLoading.value = false;
  }
}

function startPayment(entry: any) {
  payTarget.value = entry;
  paymentMethodId.value = collectableMethods.value[0]?.id ?? null;
  const currency = collectableMethods.value[0]?.currencyCode || "USD";
  paymentAmount.value =
    currency === "LBP"
      ? Math.round((Number(entry.outstanding) * exchangeRate.value) / 100)
      : Number(entry.outstanding) / 100;
}

async function submitPayment() {
  if (!payTarget.value || !paymentMethodId.value || paymentError.value) return;
  paying.value = true;
  try {
    accountDetails.value = await PayClientAccount(
      payTarget.value.invoiceId,
      paymentMethodId.value,
      paymentCents.value,
    );
    payTarget.value = null;
    const loadedClient = clients.value.find((client) => client.id === accountTarget.value?.id);
    if (loadedClient) loadedClient.accountBalance = Number(accountDetails.value?.balance || 0);
    notifyIncomeChanged();
    snackbar.text = "Account payment saved";
    snackbar.show = true;
  } catch (e: any) {
    snackbar.text = e?.toString() || "Could not save account payment";
    snackbar.show = true;
  } finally {
    paying.value = false;
  }
}

function openEdit(item: any) {
  editTarget.value = item;
  editForm.name = item.name ?? "";
  editForm.phone = item.phone ?? "";
  editOpen.value = true;
}

function openDelete(item: any) {
  deleteTarget.value = item;
  deleteOpen.value = true;
}

function openDeleteFromEdit() {
  if (!auth.isAdmin || !editTarget.value) return;
  editOpen.value = false;
  openDelete(editTarget.value);
}

async function saveEdit() {
  if (!editTarget.value) return;
  saving.value = true;
  try {
    await UpdateClient(editTarget.value.id, editForm.name.trim(), editForm.phone.trim());
    const loadedClient = clients.value.find((client) => client.id === editTarget.value.id);
    if (loadedClient) {
      loadedClient.name = editForm.name.trim();
      loadedClient.phone = editForm.phone.trim();
    }
    editOpen.value = false;
    snackbar.text = "Client updated";
    snackbar.show = true;
  } catch (e: any) {
    snackbar.text = e?.toString() || "Could not update client";
    snackbar.show = true;
  } finally {
    saving.value = false;
  }
}

async function deleteSelected() {
  if (!auth.isAdmin || !deleteTarget.value) return;
  deleting.value = true;
  try {
    await DeleteClient(deleteTarget.value.id);
    deleteOpen.value = false;
    clients.value = clients.value.filter((client) => client.id !== deleteTarget.value.id);
    if (hasMore.value) void loadMore();
    snackbar.text = "Client deleted";
    snackbar.show = true;
  } catch (e: any) {
    snackbar.text = e?.toString() || "Could not delete client";
    snackbar.show = true;
  } finally {
    deleting.value = false;
  }
}

function resetList() {
  requestSequence++;
  loading.value = false;
  clients.value = [];
  cursor.value = 0;
  hasMore.value = true;
  loadError.value = false;
  listKey.value++;
  void loadMore();
}

async function loadMore() {
  if (loading.value || !hasMore.value) return;
  const request = ++requestSequence;
  loading.value = true;
  loadError.value = false;
  try {
    const result = await ListClientsCursor(cursor.value, batchSize, debouncedSearch.value);
    if (request !== requestSequence) return;
    clients.value.push(...(result.items as ClientItem[]));
    cursor.value = result.nextCursor;
    hasMore.value = result.hasMore;
  } catch (e) {
    if (request !== requestSequence) return;
    console.error("ListClientsCursor failed:", e);
    loadError.value = true;
  } finally {
    if (request === requestSequence) loading.value = false;
  }
}

function handleScroll(event: Event) {
  const element = event.currentTarget as HTMLElement;
  if (element.scrollHeight - element.scrollTop - element.clientHeight <= 120) void loadMore();
}

watch(search, (value) => {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => {
    debouncedSearch.value = value.trim();
  }, 250);
});

watch(debouncedSearch, resetList);

watch(paymentMethodId, () => {
  if (!payTarget.value) return;
  paymentAmount.value =
    paymentCurrency.value === "LBP"
      ? Math.round((Number(payTarget.value.outstanding) * exchangeRate.value) / 100)
      : Number(payTarget.value.outstanding) / 100;
});

onMounted(resetList);

onBeforeUnmount(() => {
  clearTimeout(searchTimer);
  requestSequence++;
});
</script>

<style scoped>
.clients-page {
  display: flex;
  justify-content: center;
}

.client-search-control {
  width: 100%;
}

.client-search-row {
  padding: 12px 16px;
}

.client-list-card {
  display: flex;
  flex-direction: column;
  height: 620px;
  max-width: 760px;
  width: 100%;
}

.client-list {
  flex: 0 0 320px;
  height: 320px;
  min-height: 0;
  overflow-y: auto;
}

.client-list :deep(.v-list) {
  padding-block: 0;
}

.client-list-item {
  border-bottom: thin solid rgba(var(--v-border-color), var(--v-border-opacity));
}

.client-list-item :deep(.v-list-item__prepend) {
  padding-inline-end: 12px;
}

.client-list-item :deep(.v-list-item__append) {
  padding-inline-start: 8px;
}

.client-primary-row {
  align-items: center;
  display: flex;
  gap: 12px;
  min-width: 0;
}

.client-name {
  flex: 1 1 auto;
}

.client-phone {
  flex: 0 0 130px;
  font-size: 0.75rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.client-points {
  flex: 0 0 auto;
  font-size: 0.75rem;
}

.client-list-status {
  align-items: center;
  display: flex;
  font-size: 0.8125rem;
  justify-content: center;
  min-height: 48px;
}

.client-inline-keyboard {
  align-items: center;
  display: flex;
  flex: 0 0 205px;
  height: 205px;
  margin-bottom: 0;
  margin-inline: auto;
  max-width: 600px;
  padding-bottom: 0;
  padding-top: 12px;
}

@media (max-width: 700px) {
  .client-points,
  .client-primary-row > .v-chip {
    display: none;
  }

  .client-phone {
    flex-basis: 112px;
  }
}
</style>
