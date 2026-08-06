<template>
  <v-card class="pa-4">
    <v-card-title class="d-flex align-center justify-space-between">
      Clients
      <v-text-field
        v-model="search"
        class="ms-4"
        clearable
        density="compact"
        hide-details
        placeholder="Search"
        max-width="320"
        prepend-inner-icon="mdi-magnify"
        single-line
        variant="outlined"
      />
    </v-card-title>
    <v-data-table-server
      v-model:items-per-page="itemsPerPage"
      :headers="headers"
      :items="visibleItems"
      :items-length="totalItems"
      :loading="loading"
      density="compact"
      hover
      @update:options="loadItems"
    >
      <template #item.avatar="{ item }">
        <InitialsAvatar
          :name="item.name ?? ''"
          :color="colors[items.indexOf(item) % colors.length] ?? 'grey'"
        />
      </template>
      <template #item.totalLoyaltyPoints="{ item }">
        <span>{{ formatLoyaltyPoints(item.totalLoyaltyPoints) }}</span>
      </template>
      <template #item.phone="{ item }">
        {{ formatLebanesePhone(item.phone) }}
      </template>
      <template #item.validLoyaltyPoints="{ item }">
        <span>{{ formatLoyaltyPoints(item.validLoyaltyPoints) }}</span>
      </template>
      <template #item.accountBalance="{ item }">
        <v-chip
          :color="Number(item.accountBalance) > 0 ? 'orange-darken-2' : 'grey'"
          size="small"
          variant="tonal"
        >
          {{ formatUsd(Number(item.accountBalance || 0) / 100) }}
        </v-chip>
      </template>
      <template #item.tier="{ item }">
        <v-chip :color="loyaltyTierColor(loyaltyTier(Number(item.totalLoyaltyPoints ?? 0)))" size="small" variant="tonal">
          {{ loyaltyTier(Number(item.totalLoyaltyPoints ?? 0)) }}
        </v-chip>
      </template>
      <template #item.actions="{ item }">
        <v-btn icon="mdi-wallet" variant="text" density="comfortable" title="View on-account balance" @click.stop="openAccount(item)" />
        <template v-if="auth.isAdmin">
          <v-btn icon="mdi-pencil-outline" variant="text" density="comfortable" title="Edit client" @click.stop="openEdit(item)" />
          <v-btn icon="mdi-delete-outline" variant="text" density="comfortable" color="error" title="Delete client" @click.stop="openDelete(item)" />
        </template>
      </template>
    </v-data-table-server>
  </v-card>

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
            <tr><th>Invoice</th><th>Date</th><th class="text-end">Charged</th><th class="text-end">Paid</th><th class="text-end">Due</th><th /></tr>
          </thead>
          <tbody>
            <tr v-for="entry in accountDetails?.entries || []" :key="entry.id">
              <td>{{ entry.invoiceCode }}</td>
              <td>{{ entry.createdAt }}</td>
              <td class="text-end">{{ formatUsd(Number(entry.amount) / 100) }}</td>
              <td class="text-end">{{ formatUsd(Number(entry.paid) / 100) }}</td>
              <td class="text-end font-weight-bold">{{ formatUsd(Number(entry.outstanding) / 100) }}</td>
              <td class="text-end">
                <v-btn v-if="Number(entry.outstanding) > 0" size="small" color="primary" variant="tonal" @click="startPayment(entry)">Pay</v-btn>
                <v-chip v-else size="x-small" color="green" variant="tonal">Paid</v-chip>
              </td>
            </tr>
            <tr v-if="!accountDetails?.entries?.length"><td colspan="6" class="text-center text-medium-emphasis py-4">No on-account invoices.</td></tr>
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
              <v-btn color="primary" :loading="paying" :disabled="Boolean(paymentError) || paymentCents <= 0" @click="submitPayment">Save payment</v-btn>
            </div>
          </v-sheet>
        </v-expand-transition>

        <template v-if="accountDetails?.payments?.length">
          <div class="text-subtitle-2 mt-5 mb-2">Payment history</div>
          <v-list density="compact" border rounded>
            <v-list-item v-for="payment in accountDetails.payments" :key="payment.id">
              <template #title>{{ payment.accountName }} · {{ formatUsd(Number(payment.amount) / 100) }}</template>
              <template #subtitle>{{ payment.createdAt }} · Shift #{{ payment.shiftId }}</template>
            </v-list-item>
          </v-list>
        </template>
      </v-card-text>
      <v-card-actions><v-spacer /><v-btn variant="text" @click="accountOpen = false">Close</v-btn></v-card-actions>
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
        <v-spacer />
        <v-btn variant="text" @click="editOpen = false">Cancel</v-btn>
        <v-btn color="primary" :loading="saving" :disabled="!editForm.name.trim() || !editForm.phone.trim()" @click="saveEdit">Save</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="deleteOpen" max-width="420">
    <v-card>
      <v-card-title>Delete client?</v-card-title>
      <v-card-text>This will delete {{ deleteTarget?.name || "this client" }}. Clients linked to invoices cannot be deleted.</v-card-text>
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
import { computed, ref, onMounted, watch, reactive } from "vue";
import { DeleteClient, GetAppSettings, GetClientAccountDetails, ListClients, ListPaymentMethods, PayClientAccount, UpdateClient } from "../../wailsjs/go/main/App";
import InitialsAvatar from "@/components/InitialsAvatar.vue";
import { useAuthStore } from "@/stores/auth";
import { formatLoyaltyPoints, loyaltyTier, loyaltyTierColor } from "@/lib/client";
import { formatUsd } from "@/lib/format";
import { formatLebanesePhone } from "@/lib/phone";
import { notifyIncomeChanged } from "@/lib/incomeEvents";

const colors = ["grey-darken-1"];
const auth = useAuthStore();

const headers = [
  { title: "", key: "avatar", sortable: false, width: 56 },
  { title: "Name", key: "name" },
  { title: "Phone", key: "phone" },
  { title: "Loyalty Points", key: "totalLoyaltyPoints" },
  { title: "Tier", key: "tier" },
  { title: "Valid Points", key: "validLoyaltyPoints" },
  { title: "On Account", key: "accountBalance" },
  { title: "Created", key: "createdAt" },
  { title: "", key: "actions", sortable: false, width: 150 },
];
const items = ref<any[]>([]);
const visibleItems = ref<any[]>([]);
const search = ref("");
const itemsPerPage = ref(10);
const totalItems = ref(0);
const loading = ref(true);
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

const collectableMethods = computed(() =>
  paymentMethods.value.filter((method) => !["on account", "on coupon"].includes(String(method.name).toLowerCase())),
);
const selectedPaymentMethod = computed(() => collectableMethods.value.find((method) => method.id === paymentMethodId.value));
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
  if (paymentCents.value > Number(payTarget.value.outstanding)) return "Payment exceeds the amount due";
  return "";
});

async function openAccount(item: any) {
  accountTarget.value = item;
  accountOpen.value = true;
  payTarget.value = null;
  accountLoading.value = true;
  try {
    const [details, methods, settings] = await Promise.all([
      GetClientAccountDetails(item.id), ListPaymentMethods(), GetAppSettings(),
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
  paymentAmount.value = currency === "LBP"
    ? Math.round((Number(entry.outstanding) * exchangeRate.value) / 100)
    : Number(entry.outstanding) / 100;
}

async function submitPayment() {
  if (!payTarget.value || !paymentMethodId.value || paymentError.value) return;
  paying.value = true;
  try {
    accountDetails.value = await PayClientAccount(payTarget.value.invoiceId, paymentMethodId.value, paymentCents.value);
    payTarget.value = null;
    await loadClients();
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

async function saveEdit() {
  if (!auth.isAdmin || !editTarget.value) return;
  saving.value = true;
  try {
    await UpdateClient(editTarget.value.id, editForm.name.trim(), editForm.phone.trim());
    editOpen.value = false;
    await loadClients();
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
    await loadClients();
    snackbar.text = "Client deleted";
    snackbar.show = true;
  } catch (e: any) {
    snackbar.text = e?.toString() || "Could not delete client";
    snackbar.show = true;
  } finally {
    deleting.value = false;
  }
}

async function loadClients() {
  loading.value = true;
  try {
    items.value = await ListClients();
  } catch {
    items.value = [];
  } finally {
    loading.value = false;
  }
}

function loadItems(options: {
  page: number;
  itemsPerPage: number;
  sortBy: { key: string; order: string }[];
}) {
  let filtered = items.value;
  const query = search.value.trim().toLowerCase();
  if (query) {
    filtered = filtered.filter((item) =>
      [item.name, item.phone].some((value) =>
        String(value ?? "")
          .toLowerCase()
          .includes(query),
      ),
    );
  }
  const sort = options.sortBy[0];
  if (sort) {
    filtered = [...filtered].sort((a, b) => {
      const left = a[sort.key] ?? "";
      const right = b[sort.key] ?? "";
      return (left > right ? 1 : left < right ? -1 : 0) * (sort.order === "desc" ? -1 : 1);
    });
  }
  totalItems.value = filtered.length;
  const start = (options.page - 1) * options.itemsPerPage;
  visibleItems.value = filtered.slice(start, start + options.itemsPerPage);
}

watch([items, search, itemsPerPage], () =>
  loadItems({ page: 1, itemsPerPage: itemsPerPage.value, sortBy: [] }),
);

watch(paymentMethodId, () => {
  if (!payTarget.value) return;
  paymentAmount.value = paymentCurrency.value === "LBP"
    ? Math.round((Number(payTarget.value.outstanding) * exchangeRate.value) / 100)
    : Number(payTarget.value.outstanding) / 100;
});

onMounted(loadClients);
</script>
