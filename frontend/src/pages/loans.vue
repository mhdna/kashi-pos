<template>
  <div class="records-page" @scroll.passive.capture="list.onScroll">
    <div class="records-head">
      <div class="text-h6">Loans</div>
      <v-spacer />
      <v-btn color="primary" variant="flat" prepend-icon="mdi-plus" @click="openAdd">Add loan</v-btn>
    </div>

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
      <template #item.photo="{ item }">
        <RecordPhoto :cache-key="`loan-thumb-${item.clientRef}-${photoVersion[item.clientRef] ?? 0}`" :load="() => firstLoanPhoto(item.clientRef, 96)" />
      </template>
      <template #item.description="{ item }">
        <span class="font-weight-medium">{{ item.description || "—" }}</span>
      </template>
      <template #item.category="{ item }">
        <v-chip v-if="item.category" size="small" variant="tonal" :color="categoryOf(item.category)?.color || 'blue-grey'" :prepend-icon="categoryOf(item.category)?.icon || 'mdi-tag-outline'">
          {{ item.category }}
        </v-chip>
        <span v-else class="text-medium-emphasis">—</span>
      </template>
      <template #item.amount="{ item }">
        <span class="font-weight-bold">{{ formatCents(item.amount) }} {{ item.currencyCode }}</span>
      </template>
      <template #item.occurredAt="{ item }">{{ formatServerDateTime(item.occurredAt) }}</template>
    </v-data-table-virtual>

    <v-dialog v-model="detailOpen" max-width="700">
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

          <div class="section-label mt-5 mb-2">Photos</div>
          <div v-if="detailPhotosLoading" class="d-flex justify-center pa-6"><v-progress-circular indeterminate /></div>
          <div v-else-if="detailPhotos.length" class="photo-strip">
            <img v-for="(src, i) in detailPhotos" :key="i" :src="src" alt="Loan photo" class="photo-full" />
          </div>
          <div v-else class="empty-photos">
            <v-icon icon="mdi-image-off-outline" size="32" color="grey" />
            <div class="text-body-2 text-medium-emphasis">No photo attached{{ selected.clientRef ? "" : " (not synced yet)" }}</div>
          </div>
          <div v-if="!detailPhotosLoading && selected.clientRef && detailPhotos.length < 3" class="d-flex justify-center mt-3">
            <v-btn color="primary" variant="tonal" prepend-icon="mdi-camera-outline" @click="openUploadDialog(selected.clientRef, 'resend')">
              {{ detailPhotos.length ? "Add more photos" : "Send photo" }}
            </v-btn>
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

    <PhotoUploadDialog
      v-model="uploadOpen"
      kind="loan"
      :client-ref="uploadRef"
      :mode="uploadMode"
      @uploaded="onPhotosUploaded(uploadRef)"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount } from "vue";
import {
  CreateLoan,
  GetLoanPhoto,
  ListLoanCategories,
  ListLoanPhotoIDs,
  ListLoans,
  ListPaymentMethods,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import PhotoUploadDialog from "@/components/PhotoUploadDialog.vue";
import RecordPhoto from "@/components/RecordPhoto.vue";
import { useInfiniteList } from "@/composables/useInfiniteList";
import { formatServerDateTime } from "@/lib/datetime";
import { isSpecialPaymentMethod } from "@/lib/paymentMethods";

const headers = [
  { title: "Photo", key: "photo", sortable: false, width: 72 },
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

// The first photo, as a thumbnail, for the row; "" shows the no-photo icon.
async function firstLoanPhoto(clientRef: string, width: number): Promise<string> {
  const ids = (await ListLoanPhotoIDs(clientRef)) ?? [];
  const first = ids[0];
  return first ? GetLoanPhoto(first, width) : "";
}

const detailOpen = ref(false);
const selected = ref<any>(null);
const detailPhotos = ref<string[]>([]);
const detailPhotosLoading = ref(false);
async function openDetail(item: any) {
  selected.value = item;
  detailOpen.value = true;
  detailPhotos.value = [];
  detailPhotosLoading.value = true;
  try {
    const ids = (await ListLoanPhotoIDs(item.clientRef)) ?? [];
    const photos = await Promise.all(ids.map((id: number) => GetLoanPhoto(id, 0)));
    if (selected.value === item) detailPhotos.value = photos.filter(Boolean);
  } finally {
    detailPhotosLoading.value = false;
  }
}

// Photos: the QR right after saving, or a new link from the detail view.
const uploadOpen = ref(false);
const uploadRef = ref("");
const uploadMode = ref<"new" | "resend">("new");
// Bumped per loan when photos arrive, so its row thumbnail refetches.
const photoVersion = ref<Record<string, number>>({});
function openUploadDialog(clientRef: string, mode: "new" | "resend" = "new") {
  uploadRef.value = clientRef;
  uploadMode.value = mode;
  uploadOpen.value = true;
}
function onPhotosUploaded(clientRef: string) {
  photoVersion.value = { ...photoVersion.value, [clientRef]: (photoVersion.value[clientRef] ?? 0) + 1 };
  if (selected.value?.clientRef === clientRef && detailOpen.value) void openDetail(selected.value);
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
    const created = await CreateLoan(
      form.value.description.trim(),
      form.value.category,
      Math.round((form.value.amount ?? 0) * 100),
      selectedAccount.value?.currencyCode ?? "USD",
      form.value.cashboxAccountId as number,
    );
    dialog.value = false;
    await list.refresh();
    if (created?.syncConfigured && created?.clientRef) openUploadDialog(created.clientRef);
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
