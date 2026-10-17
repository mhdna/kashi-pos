<template>
  <div class="records-page" @scroll.passive.capture="list.onScroll">
    <div class="records-head">
      <div class="text-h6">Expenses</div>
      <v-spacer />
      <v-btn color="primary" variant="flat" prepend-icon="mdi-plus" @click="openAdd">Add expense</v-btn>
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
      no-data-text="No expenses yet."
      @click:row="(_e: unknown, row: any) => openDetail(row.item)"
    >
      <template #item.photo="{ item }">
        <RecordPhoto :cache-key="`expense-thumb-${item.clientRef}-${photoVersion[item.clientRef] ?? 0}`" :load="() => firstExpensePhoto(item.clientRef, 96)" />
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

    <!-- Detail: same shape as the invoice dialog, photos in place of items. -->
    <v-dialog v-model="detailOpen" max-width="700">
      <v-card v-if="selected">
        <v-card-title class="d-flex align-center">
          <div>
            <div class="text-h6">{{ selected.description || "Expense" }}</div>
            <div class="text-caption text-medium-emphasis">{{ formatServerDateTime(selected.occurredAt) }}</div>
          </div>
          <v-spacer />
          <v-btn icon="mdi-close" variant="text" aria-label="Close" @click="detailOpen = false" />
        </v-card-title>
        <v-card-text>
          <div class="overview-grid mb-5">
            <!-- Tap to change the category; kashi gets the change too. -->
            <v-menu location="bottom start">
              <template #activator="{ props: menuProps }">
                <button type="button" class="overview-card overview-card--edit" v-bind="menuProps" :disabled="categorySaving">
                  <v-avatar size="36" :color="categoryOf(selected.category)?.color || 'blue-grey'" variant="tonal">
                    <v-icon :icon="categoryOf(selected.category)?.icon || 'mdi-tag-outline'" size="20" />
                  </v-avatar>
                  <div class="overview-value">{{ selected.category || "No category" }}</div>
                  <v-progress-circular v-if="categorySaving" indeterminate size="16" width="2" class="ms-auto" />
                  <v-icon v-else icon="mdi-pencil-outline" size="18" class="ms-auto text-medium-emphasis" />
                </button>
              </template>
              <v-list density="compact" max-height="360">
                <v-list-item title="No category" prepend-icon="mdi-tag-outline" :active="!selected.category" @click="setCategory('')" />
                <v-list-item
                  v-for="category in expenseCategories"
                  :key="category.id"
                  :title="category.name"
                  :prepend-icon="category.icon || 'mdi-tag-outline'"
                  :base-color="category.color || undefined"
                  :active="selected.category === category.name"
                  @click="setCategory(category.name)"
                />
              </v-list>
            </v-menu>
            <div class="overview-card">
              <v-icon icon="mdi-cash" color="error" />
              <div class="overview-value">{{ formatCents(selected.amount) }} {{ selected.currencyCode }}</div>
            </div>
            <div class="overview-card">
              <v-icon icon="mdi-wallet-outline" color="primary" />
              <div class="overview-value">{{ accountName(selected.cashboxAccountId) }}</div>
            </div>
          </div>

          <div class="section-label mb-2">Photos</div>
          <div v-if="detailPhotosLoading" class="d-flex justify-center pa-6"><v-progress-circular indeterminate /></div>
          <div v-else-if="detailPhotos.length" class="photo-strip">
            <img v-for="(src, i) in detailPhotos" :key="i" :src="src" alt="Expense photo" class="photo-full" />
          </div>
          <div v-else class="empty-photos">
            <v-icon icon="mdi-image-off-outline" size="32" color="grey" />
            <div class="text-body-2 text-medium-emphasis">No photo attached{{ syncHint }}</div>
          </div>
          <!-- More photos: scan the QR with a phone, or pick files on this
               computer. kashi takes up to 3 in all. -->
          <div v-if="!detailPhotosLoading && selected.clientRef && detailPhotos.length < 3" class="d-flex justify-center flex-wrap ga-2 mt-3">
            <v-btn color="primary" variant="tonal" prepend-icon="mdi-camera-outline" @click="openUploadDialog(selected.clientRef, 'resend')">
              {{ detailPhotos.length ? "Add with phone" : "Send photo with phone" }}
            </v-btn>
            <v-btn color="primary" variant="tonal" prepend-icon="mdi-upload" :loading="computerUploading" @click="uploadFromComputer">
              From this computer
            </v-btn>
          </div>
          <div v-if="photoError" class="text-body-2 text-error text-center mt-2">{{ photoError }}</div>
        </v-card-text>
      </v-card>
    </v-dialog>

    <!-- Add: kashi's categories (with their icon and colour) as tiles, then
         description, amount in the paying account's currency, and the
         account itself. -->
    <v-dialog v-model="dialog" max-width="560">
      <v-card class="add-card">
        <v-card-title class="d-flex align-center ga-2 pt-4 px-5">
          <v-icon icon="mdi-cash-minus" color="error" />
          <span>Add expense</span>
          <v-spacer />
          <v-btn icon="mdi-close" variant="text" size="small" aria-label="Close" @click="dialog = false" />
        </v-card-title>
        <v-card-text class="px-5">
          <v-form ref="formRef">
            <div class="section-label mb-2">Category</div>
            <div v-if="expenseCategories.length" class="category-grid mb-4">
              <button
                v-for="category in expenseCategories"
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
              No expense categories yet -- they're set up in kashi and appear here after the next sync.
            </div>

            <v-text-field
              v-model="form.description"
              placeholder="Description"
              prepend-inner-icon="mdi-text"
              variant="outlined"
              density="comfortable"
              :rules="[required]"
            />
            <!-- One line per account it was paid from ($100 + 200,000 LL is
                 two lines). Each line is saved as its own expense, in that
                 account's currency, so every drawer balance stays exact. -->
            <div class="section-label mb-2">Paid from</div>
            <div v-for="(line, index) in form.payments" :key="line.key" class="payment-line mb-2">
              <v-select
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
              <v-text-field
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
              <v-btn
                v-if="form.payments.length > 1"
                icon="mdi-close"
                variant="text"
                size="small"
                aria-label="Remove this payment"
                @click="form.payments.splice(index, 1)"
              />
            </div>
            <v-btn
              v-if="form.payments.length < paymentMethods.length"
              variant="text"
              size="small"
              prepend-icon="mdi-plus"
              class="mb-1"
              @click="addPaymentLine"
            >
              Add payment method
            </v-btn>
            <div v-if="totalsLabel" class="text-body-2 font-weight-bold mt-1">Total: {{ totalsLabel }}</div>
          </v-form>
        </v-card-text>
        <v-card-actions class="px-5 pb-4">
          <v-spacer />
          <v-btn variant="text" @click="dialog = false">Cancel</v-btn>
          <v-btn color="primary" variant="flat" :loading="saving" :disabled="!canSave" @click="submit">Save expense</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <PhotoUploadDialog
      v-model="uploadOpen"
      kind="expense"
      :client-ref="uploadRef"
      :mode="uploadMode"
      @uploaded="onPhotosUploaded(uploadRef)"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount } from "vue";
import {
  AddExpensePhotosFromComputer,
  CreateExpense,
  GetExpensePhoto,
  ListExpenseCategories,
  ListExpensePhotoIDs,
  ListExpenses,
  ListPaymentMethods,
  UpdateExpenseCategory,
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

const list = useInfiniteList<any>((pageSize, pageIndex) => ListExpenses(pageSize, pageIndex));
const paymentMethods = ref<any[]>([]);
const allAccounts = ref<any[]>([]);
const expenseCategories = ref<any[]>([]);

function categoryOf(name: string) {
  return expenseCategories.value.find((c) => c.name === name);
}
function accountName(id: number) {
  return allAccounts.value.find((m) => m.id === id)?.displayName || "—";
}
function formatCents(cents: number): string {
  return (Number(cents ?? 0) / 100).toFixed(2);
}

// The first photo, as a thumbnail, for the row; "" shows the no-photo icon.
async function firstExpensePhoto(clientRef: string, width: number): Promise<string> {
  const ids = (await ListExpensePhotoIDs(clientRef)) ?? [];
  const first = ids[0];
  return first ? GetExpensePhoto(first, width) : "";
}

// ---- detail ------------------------------------------------------------
const detailOpen = ref(false);
const selected = ref<any>(null);
const detailPhotos = ref<string[]>([]);
const detailPhotosLoading = ref(false);
const syncHint = computed(() => (selected.value?.clientRef ? "" : " (not synced yet)"));

async function openDetail(item: any) {
  selected.value = item;
  photoError.value = "";
  detailOpen.value = true;
  detailPhotos.value = [];
  detailPhotosLoading.value = true;
  try {
    const ids = (await ListExpensePhotoIDs(item.clientRef)) ?? [];
    const photos = await Promise.all(ids.map((id: number) => GetExpensePhoto(id, 0)));
    if (selected.value === item) detailPhotos.value = photos.filter(Boolean);
  } finally {
    detailPhotosLoading.value = false;
  }
}

// ---- add ---------------------------------------------------------------
const dialog = ref(false);
const saving = ref(false);
const formRef = ref();
type PaymentLine = { key: number; cashboxAccountId: number | null; amount: string };
let lineKey = 0;
const form = ref({ description: "", category: "", payments: [] as PaymentLine[] });
const required = (v: unknown) => (v !== null && v !== undefined && v !== "") || "Required";

function accountOf(id: number | null) {
  return paymentMethods.value.find((m) => m.id === id);
}
// Each account once: a line can't pick one another line already uses.
function accountChoices(index: number) {
  const taken = new Set(form.value.payments.filter((_, i) => i !== index).map((p) => p.cashboxAccountId));
  return paymentMethods.value.filter((m) => !taken.has(m.id));
}
// Digits and one decimal point only (a plain text field: no spinner arrows,
// and the on-screen keypad stays numeric).
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
// Every line that was started must be complete; at least one is needed.
const canSave = computed(
  () =>
    Boolean(form.value.description?.trim()) &&
    filledLines.value.length > 0 &&
    form.value.payments.every((p) => (p.amount === "" ? false : Number(p.amount) > 0) && p.cashboxAccountId),
);

function openAdd() {
  form.value = { description: "", category: "", payments: [] };
  addPaymentLine();
  dialog.value = true;
}

// One expense per payment line, same category and description; the photo
// prompt goes with the first, since a receipt covers the whole purchase.
async function submit() {
  const valid = await formRef.value?.validate();
  if (!valid?.valid || !canSave.value) return;
  saving.value = true;
  try {
    let firstRef = "";
    let syncConfigured = false;
    for (const line of filledLines.value) {
      const created = await CreateExpense(
        form.value.description.trim(),
        form.value.category,
        Math.round(Number(line.amount) * 100),
        accountOf(line.cashboxAccountId)?.currencyCode ?? "USD",
        line.cashboxAccountId as number,
      );
      if (!firstRef && created?.clientRef) {
        firstRef = created.clientRef;
        syncConfigured = Boolean(created.syncConfigured);
      }
    }
    dialog.value = false;
    await list.refresh();
    if (syncConfigured && firstRef) {
      openUploadDialog(firstRef);
    }
  } finally {
    saving.value = false;
  }
}

// ---- photos: QR upload after saving, or a new link later -------------
const uploadOpen = ref(false);
const uploadRef = ref("");
const uploadMode = ref<"new" | "resend">("new");
// Bumped per expense when photos arrive, so its row thumbnail refetches.
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
const computerUploading = ref(false);
const photoError = ref("");

async function setCategory(name: string) {
  const item = selected.value;
  if (!item || item.category === name) return;
  categorySaving.value = true;
  try {
    await UpdateExpenseCategory(item.id, name);
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
    const sent = await AddExpensePhotosFromComputer(item.clientRef, 3 - detailPhotos.value.length);
    if (sent > 0) onPhotosUploaded(item.clientRef);
  } catch (err) {
    photoError.value = String(err);
  } finally {
    computerUploading.value = false;
  }
}

// ---- loading -------------------------------------------------------------
async function loadPaymentMethods() {
  try {
    const all = (await ListPaymentMethods()) as any[];
    allAccounts.value = all;
    paymentMethods.value = all.filter((m) => !isSpecialPaymentMethod(m.name));
  } catch {
    paymentMethods.value = [];
  }
}

async function loadExpenseCategories() {
  try {
    expenseCategories.value = (await ListExpenseCategories()) as any[];
  } catch {
    expenseCategories.value = [];
  }
}

let categorySyncUnsub: (() => void) | null = null;

onMounted(async () => {
  await Promise.all([list.loadMore(), loadPaymentMethods(), loadExpenseCategories()]);
  categorySyncUnsub = EventsOn("sync_applied:expense_categories", loadExpenseCategories);
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
</style>
