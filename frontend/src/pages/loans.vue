<template>
  <div class="records-page" @scroll.passive.capture="list.onScroll">
    <div class="records-head">
      <div class="text-h6">Loans</div>
      <v-spacer />
      <v-btn color="primary" variant="flat" prepend-icon="mdi-plus" @click="openAdd">Add loan</v-btn>
    </div>

    <!-- Loans carry no photo, so the first column is the category's icon in
         its kashi colour -- the same visual anchor the photo is elsewhere. -->
    <v-data-table-virtual
      class="records-table"
      :headers="headers"
      :items="list.items.value"
      :loading="list.loading.value"
      height="100%"
      :item-height="60"
      fixed-header
      item-value="id"
      hover
      no-data-text="No loans yet."
      @click:row="(_e: unknown, row: any) => openDetail(row.item)"
    >
      <template #item.icon="{ item }">
        <v-avatar size="40" rounded="lg" :color="categoryOf(item.category)?.color || 'blue-grey'" variant="tonal">
          <v-icon :icon="categoryOf(item.category)?.icon || 'mdi-tag-outline'" size="22" />
        </v-avatar>
      </template>
      <template #item.description="{ item }">
        <span class="font-weight-medium">{{ item.description || "—" }}</span>
      </template>
      <template #item.category="{ item }">
        <span>{{ item.category || "—" }}</span>
      </template>
      <template #item.amount="{ item }">
        <span class="font-weight-bold">{{ formatCents(item.amount) }} {{ item.currencyCode }}</span>
      </template>
      <template #item.occurredAt="{ item }">{{ formatServerDateTime(item.occurredAt) }}</template>
    </v-data-table-virtual>

    <v-dialog v-model="detailOpen" max-width="620">
      <v-card v-if="selected">
        <v-card-title class="d-flex align-center">
          <div>
            <div class="text-h6">{{ selected.description || "Loan" }}</div>
            <div class="text-caption text-medium-emphasis">{{ formatServerDateTime(selected.occurredAt) }}</div>
          </div>
          <v-spacer />
          <v-btn icon="mdi-close" variant="text" aria-label="Close" @click="detailOpen = false" />
        </v-card-title>
        <v-card-text>
          <div class="overview-grid">
            <div class="overview-card">
              <v-avatar size="36" :color="categoryOf(selected.category)?.color || 'blue-grey'" variant="tonal">
                <v-icon :icon="categoryOf(selected.category)?.icon || 'mdi-tag-outline'" size="20" />
              </v-avatar>
              <div class="overview-value">{{ selected.category || "No category" }}</div>
            </div>
            <div class="overview-card">
              <v-icon icon="mdi-cash" color="primary" />
              <div class="overview-value">{{ formatCents(selected.amount) }} {{ selected.currencyCode }}</div>
            </div>
            <div class="overview-card">
              <v-icon icon="mdi-wallet-outline" color="primary" />
              <div class="overview-value">{{ accountName(selected.cashboxAccountId) }}</div>
            </div>
          </div>
        </v-card-text>
      </v-card>
    </v-dialog>

    <v-dialog v-model="dialog" max-width="560">
      <v-card>
        <v-card-title class="d-flex align-center ga-2 pt-4 px-5">
          <v-icon icon="mdi-hand-coin-outline" color="primary" />
          <span>Add loan</span>
          <v-spacer />
          <v-btn icon="mdi-close" variant="text" size="small" aria-label="Close" @click="dialog = false" />
        </v-card-title>
        <v-card-text class="px-5">
          <v-form ref="formRef">
            <div class="section-label mb-2">Category</div>
            <div v-if="loanCategories.length" class="category-grid mb-4">
              <button
                v-for="category in loanCategories"
                :key="category.id"
                type="button"
                class="category-tile"
                :class="{ 'category-tile--on': form.category === category.name }"
                @click="form.category = category.name"
              >
                <v-avatar size="36" :color="category.color || 'blue-grey'" variant="tonal">
                  <v-icon :icon="category.icon || 'mdi-tag-outline'" size="20" />
                </v-avatar>
                <span class="category-name">{{ category.name }}</span>
              </button>
            </div>
            <div v-else class="text-body-2 text-medium-emphasis mb-4">
              No loan categories yet -- they're set up in kashi and appear here after the next sync.
            </div>

            <v-text-field
              v-model="form.description"
              label="Description"
              prepend-inner-icon="mdi-text"
              variant="outlined"
              density="comfortable"
              :rules="[required]"
            />
            <v-text-field
              v-model.number="form.amount"
              label="Amount"
              type="number"
              prepend-inner-icon="mdi-cash"
              :suffix="selectedAccount?.currencyCode || ''"
              variant="outlined"
              density="comfortable"
              :rules="[required, positive]"
            />

            <div class="section-label mb-2">Deposited to</div>
            <v-chip-group v-model="form.cashboxAccountId" mandatory selected-class="text-primary" column>
              <v-chip v-for="method in paymentMethods" :key="method.id" :value="method.id" variant="outlined" filter>
                {{ method.displayName }}
              </v-chip>
            </v-chip-group>
          </v-form>
        </v-card-text>
        <v-card-actions class="px-5 pb-4">
          <v-spacer />
          <v-btn variant="text" @click="dialog = false">Cancel</v-btn>
          <v-btn color="primary" variant="flat" :loading="saving" :disabled="!canSave" @click="submit">Save loan</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount } from "vue";
import { CreateLoan, ListLoans, ListLoanCategories, ListPaymentMethods } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { useInfiniteList } from "@/composables/useInfiniteList";
import { formatServerDateTime } from "@/lib/datetime";
import { isSpecialPaymentMethod } from "@/lib/paymentMethods";

const headers = [
  { title: "", key: "icon", sortable: false, width: 64 },
  { title: "Description", key: "description", sortable: false },
  { title: "Category", key: "category", sortable: false },
  { title: "Amount", key: "amount", sortable: false, align: "end" as const },
  { title: "Date", key: "occurredAt", sortable: false },
];

const list = useInfiniteList<any>((pageSize, pageIndex) => ListLoans(pageSize, pageIndex));
const paymentMethods = ref<any[]>([]);
const allAccounts = ref<any[]>([]);
const loanCategories = ref<any[]>([]);

function categoryOf(name: string) {
  return loanCategories.value.find((c) => c.name === name);
}
function accountName(id: number) {
  return allAccounts.value.find((m) => m.id === id)?.displayName || "—";
}
function formatCents(cents: number): string {
  return (Number(cents ?? 0) / 100).toFixed(2);
}

const detailOpen = ref(false);
const selected = ref<any>(null);
function openDetail(item: any) {
  selected.value = item;
  detailOpen.value = true;
}

const dialog = ref(false);
const saving = ref(false);
const formRef = ref();
const form = ref({ description: "", category: "", amount: null as number | null, cashboxAccountId: null as number | null });
const selectedAccount = computed(() => paymentMethods.value.find((m) => m.id === form.value.cashboxAccountId));
const required = (v: unknown) => (v !== null && v !== undefined && v !== "") || "Required";
const positive = (v: unknown) => Number(v) > 0 || "Must be more than 0";
// Loans need a category (CreateLoan's contract, as before).
const canSave = computed(
  () =>
    Boolean(form.value.description?.trim()) &&
    Boolean(form.value.category) &&
    Number(form.value.amount) > 0 &&
    Boolean(form.value.cashboxAccountId),
);

function openAdd() {
  form.value = {
    description: "",
    category: "",
    amount: null,
    cashboxAccountId: paymentMethods.value.length === 1 ? paymentMethods.value[0].id : null,
  };
  dialog.value = true;
}

async function submit() {
  const valid = await formRef.value?.validate();
  if (!valid?.valid || !canSave.value) return;
  saving.value = true;
  try {
    await CreateLoan(
      form.value.description.trim(),
      form.value.category,
      Math.round((form.value.amount ?? 0) * 100),
      selectedAccount.value?.currencyCode ?? "USD",
      form.value.cashboxAccountId as number,
    );
    dialog.value = false;
    await list.refresh();
  } finally {
    saving.value = false;
  }
}

async function loadPaymentMethods() {
  try {
    const all = (await ListPaymentMethods()) as any[];
    allAccounts.value = all;
    paymentMethods.value = all.filter((m) => !isSpecialPaymentMethod(m.name));
  } catch {
    paymentMethods.value = [];
  }
}

async function loadLoanCategories() {
  try {
    loanCategories.value = (await ListLoanCategories()) as any[];
  } catch {
    loanCategories.value = [];
  }
}

let categorySyncUnsub: (() => void) | null = null;

onMounted(async () => {
  await Promise.all([list.loadMore(), loadPaymentMethods(), loadLoanCategories()]);
  categorySyncUnsub = EventsOn("sync_applied:loan_categories", loadLoanCategories);
});

onBeforeUnmount(() => {
  categorySyncUnsub?.();
});
</script>

<style scoped>
@import "@/styles/records-page.css";

.category-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(104px, 1fr));
  gap: 8px;
}
.category-tile {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 10px 6px;
  border-radius: 12px;
  border: 1px solid rgba(var(--v-theme-on-surface), 0.12);
  background: transparent;
  cursor: pointer;
  color: inherit;
}
.category-tile--on {
  border-color: rgb(var(--v-theme-primary));
  background: rgba(var(--v-theme-primary), 0.08);
}
.category-name {
  font-size: 0.8rem;
  font-weight: 600;
  text-align: center;
  line-height: 1.15;
}
</style>
