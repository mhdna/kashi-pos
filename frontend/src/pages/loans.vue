<template>
  <div class="records-page" @scroll.passive.capture="list.onScroll">
    <div class="records-head">
      <div class="records-title">
        <k-icon icon="mdi-hand-coin-outline" size="36" color="primary" />
        <h1>Loans</h1>
      </div>
      <k-spacer />
      <k-btn color="primary" variant="flat" prepend-icon="mdi-plus" @click="openAdd">Add loan</k-btn>
    </div>

    <k-data-table-virtual
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
        <span class="font-weight-medium" :class="{ 'text-decoration-line-through': item.cancelledAt }">{{ item.description || "—" }}</span>
        <k-chip v-if="item.cancelledAt" size="x-small" color="error" variant="tonal" class="ml-2">Cancelled</k-chip>
      </template>
      <template #item.category="{ item }">
        <k-chip v-if="item.category" size="small" variant="tonal" :color="categoryOf(item.category)?.color || 'blue-grey'" :prepend-icon="categoryOf(item.category)?.icon || 'mdi-tag-outline'">
          {{ item.category }}
        </k-chip>
        <span v-else class="text-medium-emphasis">—</span>
      </template>
      <template #item.amount="{ item }">
        <span class="font-weight-bold" :class="{ 'text-decoration-line-through text-medium-emphasis': item.cancelledAt }">{{ formatCents(item.amount) }} {{ item.currencyCode }}</span>
      </template>
      <template #item.occurredAt="{ item }">{{ formatServerDateTime(item.occurredAt) }}</template>
    </k-data-table-virtual>

    <k-dialog v-model="detailOpen" max-width="700">
      <k-card v-if="selected">
        <k-card-title class="d-flex align-center">
          <div>
            <div class="text-h6">{{ selected.description || "Loan" }}</div>
            <div class="text-caption text-medium-emphasis">{{ formatServerDateTime(selected.occurredAt) }}</div>
          </div>
          <k-spacer />
          <k-btn icon="mdi-close" variant="text" aria-label="Close" @click="detailOpen = false" />
        </k-card-title>
        <k-card-text>
          <div class="overview-grid">
            <!-- Tap to change the category; kashi gets the change too. -->
            <k-menu location="bottom start">
              <template #activator="{ props: menuProps }">
                <button type="button" class="overview-card overview-card--edit" v-bind="menuProps" :disabled="categorySaving">
                  <k-avatar size="36" :color="categoryOf(selected.category)?.color || 'blue-grey'" variant="tonal">
                    <k-icon :icon="categoryOf(selected.category)?.icon || 'mdi-tag-outline'" size="20" />
                  </k-avatar>
                  <div class="overview-value">{{ selected.category || "No category" }}</div>
                  <k-progress-circular v-if="categorySaving" indeterminate size="16" width="2" class="ms-auto" />
                  <k-icon v-else icon="mdi-pencil-outline" size="18" class="ms-auto text-medium-emphasis" />
                </button>
              </template>
              <k-list density="compact" max-height="360">
                <k-list-item
                  v-for="category in loanCategories"
                  :key="category.id"
                  :title="category.name"
                  :prepend-icon="category.icon || 'mdi-tag-outline'"
                  :base-color="category.color || undefined"
                  :active="selected.category === category.name"
                  @click="setCategory(category.name)"
                />
              </k-list>
            </k-menu>
            <div class="overview-card">
              <k-icon icon="mdi-cash" color="primary" />
              <div class="overview-value">{{ formatCents(selected.amount) }} {{ selected.currencyCode }}</div>
            </div>
            <div class="overview-card">
              <k-icon icon="mdi-wallet-outline" color="primary" />
              <div class="overview-value">{{ accountName(selected.cashboxAccountId) }}</div>
            </div>
          </div>

          <div class="section-label mt-5 mb-2">Photos</div>
          <div v-if="detailPhotosLoading" class="d-flex justify-center pa-6"><k-progress-circular indeterminate /></div>
          <div v-else-if="detailPhotos.length" class="photo-strip">
            <img v-for="(src, i) in detailPhotos" :key="i" :src="src" alt="Loan photo" class="photo-full" @click="openViewer(i)" />
          </div>
          <div v-else class="empty-photos">
            <k-icon icon="mdi-image-off-outline" size="32" color="grey" />
            <div class="text-body-2 text-medium-emphasis">No photo attached{{ selected.clientRef ? "" : " (not synced yet)" }}</div>
          </div>
          <!-- More photos: scan the QR with a phone, or pick files on this
               computer. kashi takes up to 3 in all. -->
          <div v-if="!detailPhotosLoading && selected.clientRef && detailPhotos.length < 3" class="d-flex justify-center flex-wrap ga-2 mt-3">
            <k-btn color="primary" variant="tonal" prepend-icon="mdi-camera-outline" @click="openUploadDialog(selected.clientRef, 'resend')">
              {{ detailPhotos.length ? "Add with phone" : "Send photo with phone" }}
            </k-btn>
            <k-btn color="primary" variant="tonal" prepend-icon="mdi-upload" :loading="computerUploading" @click="uploadFromComputer">
              From this computer
            </k-btn>
          </div>
          <div v-if="photoError" class="text-body-2 text-error text-center mt-2">{{ photoError }}</div>
        </k-card-text>
        <k-card-actions v-if="auth.isAdmin && !selected.cancelledAt" class="px-5 pb-4">
          <k-spacer />
          <k-btn color="error" variant="tonal" prepend-icon="mdi-cancel" @click="requestCancel">Cancel loan</k-btn>
        </k-card-actions>
      </k-card>
    </k-dialog>

    <k-dialog v-model="cancelOpen" max-width="420">
      <k-card v-if="selected">
        <k-card-title class="pt-4 px-5">Cancel this loan?</k-card-title>
        <k-card-text class="px-5">
          "{{ selected.description || "Loan" }}" ({{ formatCents(selected.amount) }} {{ selected.currencyCode }}) stays in the list marked
          cancelled, and the shift and kashi stop counting it.
          <div v-if="cancelError" class="text-body-2 text-error mt-2">{{ cancelError }}</div>
        </k-card-text>
        <k-card-actions class="px-5 pb-4">
          <k-spacer />
          <k-btn variant="text" @click="cancelOpen = false">Keep it</k-btn>
          <k-btn color="error" variant="flat" :loading="cancelling" @click="cancelSelected">Cancel loan</k-btn>
        </k-card-actions>
      </k-card>
    </k-dialog>

    <k-dialog v-model="dialog" max-width="720">
      <k-card>
        <k-card-title class="d-flex align-center ga-2 pt-4 px-5">
          <k-icon icon="mdi-hand-coin-outline" color="primary" />
          <span>Add loan</span>
          <k-spacer />
          <k-btn icon="mdi-close" variant="text" size="small" aria-label="Close" @click="dialog = false" />
        </k-card-title>
        <k-card-text class="px-5">
          <k-form ref="formRef">
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
                <k-avatar size="36" :color="category.color || 'blue-grey'" variant="tonal">
                  <k-icon :icon="category.icon || 'mdi-tag-outline'" size="20" />
                </k-avatar>
                <span class="category-name">{{ category.name }}</span>
              </button>
            </div>
            <div v-else class="text-body-2 text-medium-emphasis mb-4">
              No loan categories yet -- they're set up in kashi and appear here after the next sync.
            </div>

            <k-text-field
              v-model="form.description"
              placeholder="Description"
              prepend-inner-icon="mdi-text"
              variant="outlined"
              density="comfortable"
              :rules="[required]"
            />
            <!-- One line per account it went into ($100 + 200,000 LL is two
                 lines), each saved as its own loan in that account's
                 currency, as Add expense does. -->
            <div class="section-label mb-2">Deposited to</div>
            <div v-for="(line, index) in form.payments" :key="line.key" class="payment-line mb-2">
              <k-select
                v-model="line.cashboxAccountId"
                :items="accountChoices(index)"
                item-title="displayName"
                item-value="id"
                placeholder="Account"
                variant="outlined"
                density="comfortable"
                hide-details
                class="payment-account"
              />
              <k-text-field
                v-model="line.amount"
                placeholder="Amount"
                inputmode="decimal"
                :suffix="accountOf(line.cashboxAccountId)?.currencyCode || ''"
                variant="outlined"
                density="comfortable"
                hide-details
                class="payment-amount"
                @update:model-value="(v: string) => (line.amount = cleanAmount(v))"
              />
              <k-btn
                v-if="form.payments.length > 1"
                icon="mdi-close"
                variant="text"
                size="small"
                aria-label="Remove this account"
                @click="form.payments.splice(index, 1)"
              />
            </div>
            <k-btn
              v-if="form.payments.length < paymentMethods.length"
              variant="text"
              size="small"
              prepend-icon="mdi-plus"
              class="mb-1"
              @click="addPaymentLine"
            >
              Add payment method
            </k-btn>
            <div v-if="totalsLabel" class="text-body-2 font-weight-bold mt-1">Total: {{ totalsLabel }}</div>
          </k-form>
        </k-card-text>
        <k-card-actions class="px-5 pb-4">
          <k-spacer />
          <k-btn variant="text" @click="dialog = false">Cancel</k-btn>
          <k-btn color="primary" variant="flat" :loading="saving" :disabled="!canSave" @click="submit">Save loan</k-btn>
        </k-card-actions>
      </k-card>
    </k-dialog>

    <PhotoViewer v-model="viewerOpen" v-model:index="viewerIndex" :photos="detailPhotos" />
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
  AddLoanPhotosFromComputer,
  CancelLoan,
  CreateLoan,
  GetLoanFirstPhoto,
  GetLoanPhoto,
  ListLoanCategories,
  ListLoanPhotoIDs,
  ListLoans,
  ListPaymentMethods,
  UpdateLoanCategory,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import PhotoUploadDialog from "@/components/PhotoUploadDialog.vue";
import PhotoViewer from "@/components/PhotoViewer.vue";
import RecordPhoto from "@/components/RecordPhoto.vue";
import { detailPhotoWidth } from "@/lib/photos";
import { useInfiniteList } from "@/composables/useInfiniteList";
import { formatServerDateTime } from "@/lib/datetime";
import { isSpecialPaymentMethod } from "@/lib/paymentMethods";
import { notifyIncomeChanged } from "@/lib/incomeEvents";
import { useAuthStore } from "../stores/auth";

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
  return GetLoanFirstPhoto(clientRef, width);
}

const detailOpen = ref(false);
const selected = ref<any>(null);
const detailPhotos = ref<string[]>([]);
const detailPhotosLoading = ref(false);
async function openDetail(item: any) {
  selected.value = item;
  photoError.value = "";
  detailOpen.value = true;
  detailPhotos.value = [];
  detailPhotosLoading.value = true;
  try {
    const ids = (await ListLoanPhotoIDs(item.clientRef)) ?? [];
    const photos = await Promise.all(ids.map((id: number) => GetLoanPhoto(id, detailPhotoWidth)));
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

// ---- detail edits: category, photos from this computer -------------------
const categorySaving = ref(false);
const viewerOpen = ref(false);
const viewerIndex = ref(0);
function openViewer(i: number) {
  viewerIndex.value = i;
  viewerOpen.value = true;
}
const computerUploading = ref(false);
const photoError = ref("");

async function setCategory(name: string) {
  const item = selected.value;
  if (!item || item.category === name) return;
  categorySaving.value = true;
  try {
    await UpdateLoanCategory(item.id, name);
    // The row in the table is this same object, so it updates too.
    item.category = name;
  } catch (err) {
    photoError.value = `Couldn't change the category: ${err}`;
  } finally {
    categorySaving.value = false;
  }
}

async function uploadFromComputer() {
  const item = selected.value;
  if (!item?.clientRef) return;
  photoError.value = "";
  computerUploading.value = true;
  try {
    const sent = await AddLoanPhotosFromComputer(item.clientRef, 3 - detailPhotos.value.length);
    if (sent > 0) onPhotosUploaded(item.clientRef);
  } catch (err) {
    photoError.value = String(err);
  } finally {
    computerUploading.value = false;
  }
}

// Cancelling keeps the row, marked cancelled, and tells kashi (admin only).
const auth = useAuthStore();
const cancelOpen = ref(false);
const cancelling = ref(false);
const cancelError = ref("");
function requestCancel() {
  cancelError.value = "";
  cancelOpen.value = true;
}
async function cancelSelected() {
  const item = selected.value;
  if (!item || !auth.isAdmin) return;
  cancelling.value = true;
  cancelError.value = "";
  try {
    await CancelLoan(item.id);
    notifyIncomeChanged();
    cancelOpen.value = false;
    detailOpen.value = false;
    await list.refresh();
  } catch (err) {
    cancelError.value = String(err);
  } finally {
    cancelling.value = false;
  }
}

const dialog = ref(false);
const saving = ref(false);
const formRef = ref();
type PaymentLine = { key: number; cashboxAccountId: number | null; amount: string };
let lineKey = 0;
const form = ref({ description: "", category: "", payments: [] as PaymentLine[] });
const required = (v: unknown) => (v !== null && v !== undefined && v !== "") || "Required";

// The payment lines work exactly as on Add expense (expenses.vue).
function accountOf(id: number | null) {
  return paymentMethods.value.find((m) => m.id === id);
}
// Each account once: a line can't pick one another line already uses.
function accountChoices(index: number) {
  const taken = new Set(form.value.payments.filter((_, i) => i !== index).map((p) => p.cashboxAccountId));
  return paymentMethods.value.filter((m) => !taken.has(m.id));
}
function cleanAmount(value: string) {
  const digits = String(value ?? "").replace(/[^\d.]/g, "");
  const [whole = "", ...rest] = digits.split(".");
  return rest.length ? `${whole}.${rest.join("")}` : whole;
}
function addPaymentLine() {
  const next = accountChoices(form.value.payments.length)[0];
  form.value.payments.push({ key: ++lineKey, cashboxAccountId: next?.id ?? null, amount: "" });
}
const filledLines = computed(() =>
  form.value.payments.filter((p) => p.cashboxAccountId && Number(p.amount) > 0),
);
const totalsLabel = computed(() => {
  const byCurrency = new Map<string, number>();
  for (const p of filledLines.value) {
    const code = accountOf(p.cashboxAccountId)?.currencyCode || "";
    byCurrency.set(code, (byCurrency.get(code) ?? 0) + Number(p.amount));
  }
  return [...byCurrency].map(([code, sum]) => `${sum.toLocaleString()} ${code}`.trim()).join(" + ");
});
// Loans need a category (CreateLoan's contract, as before), and every line
// that was started must be complete.
const canSave = computed(
  () =>
    Boolean(form.value.description?.trim()) &&
    Boolean(form.value.category) &&
    filledLines.value.length > 0 &&
    form.value.payments.every((p) => Number(p.amount) > 0 && p.cashboxAccountId),
);

function openAdd() {
  form.value = { description: "", category: "", payments: [] };
  addPaymentLine();
  dialog.value = true;
}

// One loan per line, same category and description; the photo prompt goes
// with the first.
async function submit() {
  const valid = await formRef.value?.validate();
  if (!valid?.valid || !canSave.value) return;
  saving.value = true;
  try {
    let first: any = null;
    for (const line of filledLines.value) {
      const created = await CreateLoan(
        form.value.description.trim(),
        form.value.category,
        Math.round(Number(line.amount) * 100),
        accountOf(line.cashboxAccountId)?.currencyCode ?? "USD",
        line.cashboxAccountId as number,
      );
      if (!first && created?.clientRef) first = created;
    }
    dialog.value = false;
    await list.refresh();
    if (first?.syncConfigured) openUploadDialog(first.clientRef);
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
let recordSyncUnsub: (() => void) | null = null;

onMounted(async () => {
  await Promise.all([list.loadMore(), loadPaymentMethods(), loadLoanCategories()]);
  categorySyncUnsub = EventsOn("sync_applied:loan_categories", loadLoanCategories);
  recordSyncUnsub = EventsOn("sync_applied:loans", () => void list.refresh());
});

onBeforeUnmount(() => {
  categorySyncUnsub?.();
  recordSyncUnsub?.();
});
</script>

<style scoped>
@import "@/styles/records-page.css";

.payment-line {
  display: flex;
  align-items: center;
  gap: 8px;
}
.payment-account {
  flex: 1 1 55%;
}
.payment-amount {
  flex: 1 1 45%;
}

.category-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(104px, 1fr));
  gap: 8px;
  /* The tiles scroll in their own block, so the dialog stays as tall as the form needs. */
  max-height: 330px;
  overflow-y: auto;
  padding: 2px;
}
.category-tile {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 10px 6px;
  border-radius: 12px;
  border: 1px solid rgba(var(--k-theme-on-surface), 0.12);
  background: transparent;
  cursor: pointer;
  color: inherit;
}
.category-tile--on {
  border-color: rgb(var(--k-theme-primary));
  background: rgba(var(--k-theme-primary), 0.08);
}
.category-name {
  font-size: 0.8rem;
  font-weight: 600;
  text-align: center;
  line-height: 1.15;
}
</style>
