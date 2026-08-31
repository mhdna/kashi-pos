<template>
  <v-card class="pa-4">
    <v-card-title class="d-flex align-center justify-space-between">
      Expenses
      <v-btn color="primary" prepend-icon="mdi-plus" @click="dialog = true">Add expense</v-btn>
    </v-card-title>
    <v-data-table :headers="headers" :items="items" :loading="loading" density="compact" hover>
      <template #item.amount="{ item }">{{ formatCents(item.amount) }} {{ item.currencyCode }}</template>
      <template #item.occurredAt="{ item }">{{ formatServerDateTime(item.occurredAt) }}</template>
    </v-data-table>

    <v-dialog v-model="dialog" max-width="480">
      <v-card>
        <v-card-title>Add expense</v-card-title>
        <v-card-text>
          <v-form ref="formRef" v-model="formValid">
            <v-text-field v-model="form.description" label="Description" :rules="[required]" />
            <v-combobox
              v-model="form.category"
              :items="expenseCategories"
              item-title="name"
              item-value="name"
              :return-object="false"
              label="Category"
              clearable
            >
              <template #item="{ props: itemProps, item }">
                <v-list-item v-bind="itemProps" :prepend-icon="item.raw.icon || 'mdi-tag-outline'" />
              </template>
            </v-combobox>
            <v-text-field v-model.number="form.amount" label="Amount" type="number" :rules="[required]" />
            <v-select
              v-model="form.cashboxAccountId"
              :items="paymentMethods"
              item-title="displayName"
              item-value="id"
              label="Paid from"
              :rules="[required]"
            />
          </v-form>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="dialog = false">Cancel</v-btn>
          <v-btn color="primary" :loading="saving" @click="submit">Save</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="uploadDialog" max-width="380" persistent @update:model-value="onUploadDialogVisibilityChange">
      <v-card class="text-center pa-4">
        <template v-if="uploadState === 'waiting'">
          <v-card-title>Preparing upload link…</v-card-title>
          <v-card-text>
            <v-progress-circular class="my-4" color="primary" indeterminate size="48" />
            <div class="text-body-2">Syncing this expense to the server.</div>
          </v-card-text>
        </template>

        <template v-else-if="uploadState === 'ready'">
          <v-card-title>Attach a receipt photo</v-card-title>
          <v-card-text>
            <div class="text-body-2 mb-3">Scan this with a phone to upload up to 3 photos.</div>
            <img v-if="qrDataUrl" :src="qrDataUrl" alt="Upload QR code" style="width: 220px; height: 220px" />
            <div class="text-caption text-medium-emphasis mt-3" style="word-break: break-all">{{ uploadUrl }}</div>
          </v-card-text>
        </template>

        <template v-else-if="uploadState === 'success'">
          <v-card-text>
            <v-icon class="my-2" color="success" icon="mdi-check-circle" size="72" />
            <div class="text-h6">Uploaded!</div>
          </v-card-text>
        </template>

        <template v-else-if="uploadState === 'error'">
          <v-card-title>Couldn't prepare the upload link</v-card-title>
          <v-card-text>
            <div class="text-body-2">Check the connection to the server and try again later.</div>
          </v-card-text>
        </template>

        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="closeUploadDialog">Close</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-card>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from "vue";
import QRCode from "qrcode";
import { CreateExpense, ListExpenses, ListExpenseCategories, ListPaymentMethods, CheckExpenseUploadStatus } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { formatServerDateTime } from "@/lib/datetime";
import { isSpecialPaymentMethod } from "@/lib/paymentMethods";

const headers = [
  { title: "ID", key: "id" },
  { title: "Description", key: "description" },
  { title: "Category", key: "category" },
  { title: "Amount", key: "amount" },
  { title: "Date", key: "occurredAt" },
];
const items = ref<any[]>([]);
const loading = ref(true);
const paymentMethods = ref<any[]>([]);
const expenseCategories = ref<any[]>([]);

const dialog = ref(false);
const saving = ref(false);
const formValid = ref(false);
const formRef = ref();
const form = ref({ description: "", category: "", amount: null as number | null, cashboxAccountId: null as number | null });

const required = (v: unknown) => (v !== null && v !== undefined && v !== "") || "Required";

type UploadState = "waiting" | "ready" | "success" | "error";

const uploadDialog = ref(false);
const uploadState = ref<UploadState>("waiting");
const uploadUrl = ref("");
const qrDataUrl = ref("");
let pendingClientRef = "";
let uploadUnsubscribers: Array<() => void> = [];
let pollHandle: ReturnType<typeof setInterval> | null = null;
let waitingTimeoutHandle: ReturnType<typeof setTimeout> | null = null;

const UPLOAD_READY_TIMEOUT_MS = 30000;
const UPLOAD_POLL_INTERVAL_MS = 4000;

function stopUploadWatchers() {
  uploadUnsubscribers.forEach((unsub) => unsub());
  uploadUnsubscribers = [];
  if (pollHandle !== null) {
    clearInterval(pollHandle);
    pollHandle = null;
  }
  if (waitingTimeoutHandle !== null) {
    clearTimeout(waitingTimeoutHandle);
    waitingTimeoutHandle = null;
  }
}

function markUploadSuccess() {
  uploadState.value = "success";
  stopUploadWatchers();
}

function startUploadPolling(clientRef: string) {
  pollHandle = setInterval(async () => {
    try {
      const uploaded = await CheckExpenseUploadStatus(clientRef);
      if (uploaded) markUploadSuccess();
    } catch {
      // Transient network hiccups are expected while a branch is offline —
      // the WS push (or the next poll tick) will still get there.
    }
  }, UPLOAD_POLL_INTERVAL_MS);
}

async function openUploadDialog(clientRef: string) {
  stopUploadWatchers();
  pendingClientRef = clientRef;
  uploadUrl.value = "";
  qrDataUrl.value = "";
  uploadState.value = "waiting";
  uploadDialog.value = true;

  waitingTimeoutHandle = setTimeout(() => {
    if (uploadState.value === "waiting") {
      uploadState.value = "error";
      stopUploadWatchers();
    }
  }, UPLOAD_READY_TIMEOUT_MS);

  uploadUnsubscribers.push(
    EventsOn(`expense_upload_ready:${clientRef}`, async (url: string) => {
      if (waitingTimeoutHandle !== null) {
        clearTimeout(waitingTimeoutHandle);
        waitingTimeoutHandle = null;
      }
      uploadUrl.value = url;
      try {
        qrDataUrl.value = await QRCode.toDataURL(url, { width: 220, margin: 1 });
      } catch {
        qrDataUrl.value = "";
      }
      uploadState.value = "ready";
      startUploadPolling(clientRef);
    }),
  );
  uploadUnsubscribers.push(EventsOn(`expense_images_uploaded:${clientRef}`, markUploadSuccess));
}

function closeUploadDialog() {
  uploadDialog.value = false;
}

function onUploadDialogVisibilityChange(visible: boolean) {
  if (!visible) stopUploadWatchers();
}

onBeforeUnmount(stopUploadWatchers);

function formatCents(cents: number): string {
  return (Number(cents ?? 0) / 100).toFixed(2);
}

async function loadExpenses() {
  loading.value = true;
  try {
    items.value = await ListExpenses(50, 0);
  } catch {
    items.value = [];
  }
  loading.value = false;
}

async function loadPaymentMethods() {
  try {
    const all = (await ListPaymentMethods()) as any[];
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

async function submit() {
  const valid = await formRef.value?.validate();
  if (!valid?.valid) return;

  const account = paymentMethods.value.find((m) => m.id === form.value.cashboxAccountId);
  saving.value = true;
  try {
    const created = await CreateExpense(
      form.value.description,
      form.value.category,
      Math.round((form.value.amount ?? 0) * 100),
      account?.currencyCode ?? "USD",
      form.value.cashboxAccountId as number,
    );
    dialog.value = false;
    form.value = { description: "", category: "", amount: null, cashboxAccountId: null };
    await loadExpenses();
    if (created?.syncConfigured && created?.clientRef) {
      openUploadDialog(created.clientRef);
    }
  } finally {
    saving.value = false;
  }
}

let categorySyncUnsub: (() => void) | null = null;

onMounted(async () => {
  await Promise.all([loadExpenses(), loadPaymentMethods(), loadExpenseCategories()]);
  categorySyncUnsub = EventsOn("sync_applied:expense_categories", loadExpenseCategories);
});

onBeforeUnmount(() => {
  categorySyncUnsub?.();
});
</script>
