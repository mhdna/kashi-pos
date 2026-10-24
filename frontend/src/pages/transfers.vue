<template>
  <div class="records-page">
    <div class="records-head">
      <div class="text-h6">Transfers</div>
      <v-tabs v-model="tab" density="compact" class="ms-4">
        <v-tab value="outgoing">Outgoing</v-tab>
        <v-tab value="incoming">
          Incoming
          <v-badge v-if="unconfirmedIncomingCount > 0" color="warning" :content="unconfirmedIncomingCount" inline />
        </v-tab>
      </v-tabs>
      <v-spacer />
      <v-btn v-if="tab === 'outgoing'" color="primary" variant="flat" prepend-icon="mdi-plus" @click="openCreate">
        New Transfer
      </v-btn>
    </div>

    <!-- Same layout as the invoices list; the transfer's photo (taken with a
         phone, kept in kashi) is the first column. -->
    <v-data-table-virtual
      v-if="tab === 'outgoing'"
      class="records-table"
      :headers="outgoingHeaders"
      :items="outgoing"
      :loading="outgoingLoading"
      height="100%"
      :item-height="60"
      fixed-header
      item-value="id"
      hover
      no-data-text="No transfers sent yet."
      @click:row="(_e: unknown, row: any) => openOutgoing(row.item)"
    >
      <template #item.photo="{ item }">
        <RecordPhoto :cache-key="`transfer-out-${item.clientRef}`" :load="() => GetTransferPhoto(item.clientRef, 0, 96)" />
      </template>
      <template #item.toBranchCode="{ item }">
        <span class="font-weight-medium">{{ branchName(item.toBranchCode) }}</span>
      </template>
      <template #item.note="{ item }">
        <span class="text-truncate d-inline-block" style="max-width: 260px">{{ item.note || "—" }}</span>
      </template>
      <template #item.status="{ item }">
        <v-chip :color="statusColor(item.status)" size="small">{{ item.status }}</v-chip>
      </template>
      <template #item.createdAt="{ item }">{{ formatServerDateTime(item.createdAt) }}</template>
    </v-data-table-virtual>

    <v-data-table-virtual
      v-else
      class="records-table"
      :headers="incomingHeaders"
      :items="incoming"
      :loading="incomingLoading"
      height="100%"
      :item-height="60"
      fixed-header
      item-value="id"
      hover
      no-data-text="No incoming transfers."
      @click:row="(_e: unknown, row: any) => openIncoming(row.item)"
    >
      <template #item.photo="{ item }">
        <RecordPhoto :cache-key="`transfer-in-${item.id}`" :load="() => GetTransferPhoto('', item.id, 96)" />
      </template>
      <template #item.senderBranchName="{ item }">
        <span class="font-weight-medium">{{ item.senderBranchName || "—" }}</span>
      </template>
      <template #item.note="{ item }">
        <span class="text-truncate d-inline-block" style="max-width: 260px">{{ item.note || "—" }}</span>
      </template>
      <template #item.status="{ item }">
        <v-chip :color="statusColor(item.status)" size="small">{{ item.status }}</v-chip>
      </template>
      <template #item.confirmed="{ item }">
        <v-icon v-if="isConfirmed(item)" color="success" icon="mdi-check-circle" />
        <v-icon v-else color="warning" icon="mdi-clock-outline" />
      </template>
      <template #item.createdAt="{ item }">{{ formatServerDateTime(item.createdAt) }}</template>
    </v-data-table-virtual>

    <!-- Outgoing detail: the invoice dialog's shape, photo then items. -->
    <v-dialog v-model="outgoingDialog" max-width="700">
      <v-card v-if="outgoingTarget">
        <v-card-title class="d-flex align-center">
          <div>
            <div class="text-h6">To {{ branchName(outgoingTarget.toBranchCode) }}</div>
            <div class="text-caption text-medium-emphasis">{{ formatServerDateTime(outgoingTarget.createdAt) }}</div>
          </div>
          <v-spacer />
          <v-chip :color="statusColor(outgoingTarget.status)" size="small" class="me-2">{{ outgoingTarget.status }}</v-chip>
          <v-btn icon="mdi-close" variant="text" aria-label="Close" @click="outgoingDialog = false" />
        </v-card-title>
        <v-card-text>
          <div v-if="outgoingTarget.note" class="mb-4">{{ outgoingTarget.note }}</div>
          <div class="section-label mb-2">Photo</div>
          <div v-if="detailPhotoLoading" class="d-flex justify-center pa-6"><v-progress-circular indeterminate /></div>
          <img v-else-if="detailPhoto" :src="detailPhoto" alt="Transfer photo" class="photo-full mb-4" @click="viewerOpen = true" />
          <div v-else class="empty-photos mb-2">
            <v-icon icon="mdi-image-off-outline" size="32" color="grey" />
            <div class="text-body-2 text-medium-emphasis">No photo attached</div>
          </div>
          <div class="section-label mb-2">Items</div>
          <v-list density="compact" class="pa-0">
            <v-list-item v-for="line in outgoingTarget.items || []" :key="line.id || line.barcode">
              <template #title>{{ line.name || line.barcode }}</template>
              <template #append><span class="font-weight-bold">× {{ line.quantity }}</span></template>
            </v-list-item>
            <div v-if="!(outgoingTarget.items || []).length" class="text-body-2 text-medium-emphasis">No items listed.</div>
          </v-list>
        </v-card-text>
      </v-card>
    </v-dialog>

    <!-- --------------------------------------------------------- CREATE -->
    <v-dialog v-model="createDialog" max-width="560">
      <v-card>
        <v-card-title>New Transfer</v-card-title>
        <v-card-text>
          <v-select
            v-model="createForm.toBranchCode"
            item-title="name"
            item-value="code"
            :items="branches"
            placeholder="Destination branch"
            :rules="[required]"
          />
          <v-text-field v-model="createForm.note" placeholder="Note (optional)" />

          <div class="text-subtitle-2 mt-4 mb-1">Items</div>
          <div class="d-flex align-center mb-2" style="gap: 12px">
            <v-text-field
              v-model="newItemBarcode"
              density="compact"
              hide-details
              placeholder="Scan or type barcode"
              @keyup.enter="addCreateItem"
            />
            <v-text-field
              v-model.number="newItemQuantity"
              density="compact"
              hide-details
              placeholder="Qty"
              style="max-width: 100px"
              type="number"
            />
            <v-btn :disabled="!newItemBarcode || !newItemQuantity" @click="addCreateItem">Add</v-btn>
          </div>
          <v-table density="compact">
            <thead>
              <tr>
                <th>Barcode</th>
                <th class="text-end">Qty</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(item, i) in createForm.items" :key="i">
                <td>{{ item.Barcode }}</td>
                <td class="text-end">{{ item.Quantity }}</td>
                <td class="text-end">
                  <v-icon icon="mdi-close" size="small" @click="createForm.items.splice(i, 1)" />
                </td>
              </tr>
            </tbody>
          </v-table>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="createDialog = false">Cancel</v-btn>
          <v-btn
            color="primary"
            :disabled="!createForm.toBranchCode || createForm.items.length === 0"
            :loading="creating"
            @click="submitCreate"
          >
            Send
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- ----------------------------------------------------- UPLOAD QR -->
    <v-dialog v-model="uploadDialog" max-width="380" persistent @update:model-value="onUploadDialogVisibilityChange">
      <v-card class="text-center pa-4">
        <template v-if="uploadState === 'waiting'">
          <v-card-title>Preparing upload link…</v-card-title>
          <v-card-text>
            <v-progress-circular class="my-4" color="primary" indeterminate size="48" />
            <div class="text-body-2">Syncing this transfer to the server.</div>
          </v-card-text>
        </template>

        <template v-else-if="uploadState === 'ready'">
          <v-card-title>Attach a photo (optional)</v-card-title>
          <v-card-text>
            <div class="text-body-2">Scan this with a phone to upload one photo of the transfer.</div>
            <div class="text-body-2 mb-3" dir="rtl" lang="ar">امسح الرمز بالهاتف والتقط صورة واحدة لورقة التحويل.</div>
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

    <!-- ------------------------------------------------------- CONFIRM -->
    <v-dialog v-model="incomingDialog" max-width="640">
      <v-card v-if="incomingTarget">
        <v-card-title>
          From {{ incomingTarget.senderBranchName }}
          <v-chip class="ml-2" :color="statusColor(incomingTarget.status)" size="small">{{ incomingTarget.status }}</v-chip>
        </v-card-title>
        <v-card-text>
          <div v-if="incomingTarget.note" class="text-body-2 text-medium-emphasis mb-2">{{ incomingTarget.note }}</div>
          <img v-if="detailPhoto" :src="detailPhoto" alt="Transfer photo" class="photo-full mb-3" @click="viewerOpen = true" />
          <v-table density="compact">
            <thead>
              <tr>
                <th>Barcode</th>
                <th class="text-end">Sent</th>
                <th class="text-end">Counted</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in incomingItems" :key="item.id">
                <td>{{ item.barcode }}</td>
                <td class="text-end">{{ item.quantity }}</td>
                <td class="text-end">
                  <v-text-field
                    v-model.number="item.countedQuantity"
                    density="compact"
                    hide-details
                    style="max-width: 100px; display: inline-block"
                    type="number"
                  />
                </td>
              </tr>
            </tbody>
          </v-table>
          <v-text-field v-model="incomingNote" class="mt-4" placeholder="Note (optional) — e.g. damage, shortage" />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="incomingDialog = false">Close</v-btn>
          <v-btn color="primary" :loading="confirming" @click="submitConfirm">Confirm receipt</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-snackbar v-model="snackbar.show" timeout="3500">{{ snackbar.text }}</v-snackbar>
    <PhotoViewer v-model="viewerOpen" :photos="detailPhoto ? [detailPhoto] : []" />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount } from "vue";
import QRCode from "qrcode";
import {
  ListTransferRequests,
  ListIncomingTransferRequests,
  ListDestinationBranches,
  CreateTransferRequest,
  ConfirmTransferRequest,
  CheckTransferUploadStatus,
  GetTransferPhoto,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import RecordPhoto from "@/components/RecordPhoto.vue";
import { detailPhotoWidth } from "@/lib/photos";
import PhotoViewer from "@/components/PhotoViewer.vue";
import { formatServerDateTime } from "@/lib/datetime";

const tab = ref("outgoing");
const required = (v: unknown) => !!v || "Required";
const snackbar = reactive({ show: false, text: "" });

function statusColor(s: string) {
  return (
    ({
      requested: "info",
      draft: "grey",
      dispatched: "warning",
      received: "success",
      cancelled: "error",
    } as Record<string, string>)[s] || "grey"
  );
}

// ------------------------------------------------------------ OUTGOING
const outgoingHeaders = [
  { title: "Photo", key: "photo", sortable: false, width: 72 },
  { title: "To", key: "toBranchCode", sortable: false },
  { title: "Note", key: "note", sortable: false },
  { title: "Status", key: "status", sortable: false },
  { title: "Created", key: "createdAt", sortable: false },
];

function branchName(code: string) {
  return branches.value.find((b) => b.code === code)?.name || code || "—";
}

// The photo for whichever transfer's dialog is open, full size.
const detailPhoto = ref("");
const viewerOpen = ref(false);
const detailPhotoLoading = ref(false);
async function loadDetailPhoto(clientRef: string, transferID: number) {
  detailPhoto.value = "";
  detailPhotoLoading.value = true;
  try {
    detailPhoto.value = await GetTransferPhoto(clientRef, transferID, detailPhotoWidth);
  } finally {
    detailPhotoLoading.value = false;
  }
}

const outgoingDialog = ref(false);
const outgoingTarget = ref<any>(null);
function openOutgoing(item: any) {
  outgoingTarget.value = item;
  outgoingDialog.value = true;
  void loadDetailPhoto(item.clientRef, 0);
}
const outgoing = ref<any[]>([]);
const outgoingLoading = ref(true);

async function loadOutgoing() {
  outgoingLoading.value = true;
  try {
    outgoing.value = await ListTransferRequests();
  } catch {
    outgoing.value = [];
  }
  outgoingLoading.value = false;
}

const branches = ref<any[]>([]);
async function loadBranches() {
  try {
    branches.value = await ListDestinationBranches();
  } catch {
    branches.value = [];
  }
}

const createDialog = ref(false);
const creating = ref(false);
const createForm = ref<{ toBranchCode: string | null; note: string; items: { Barcode: string; Quantity: number }[] }>({
  toBranchCode: null,
  note: "",
  items: [],
});
const newItemBarcode = ref("");
const newItemQuantity = ref<number | null>(1);

function openCreate() {
  createForm.value = { toBranchCode: null, note: "", items: [] };
  newItemBarcode.value = "";
  newItemQuantity.value = 1;
  createDialog.value = true;
}

function addCreateItem() {
  if (!newItemBarcode.value || !newItemQuantity.value) return;
  createForm.value.items.push({ Barcode: newItemBarcode.value, Quantity: newItemQuantity.value });
  newItemBarcode.value = "";
  newItemQuantity.value = 1;
}

async function submitCreate() {
  if (!createForm.value.toBranchCode || createForm.value.items.length === 0) return;
  creating.value = true;
  try {
    const created = await CreateTransferRequest(createForm.value.toBranchCode, createForm.value.note, createForm.value.items);
    createDialog.value = false;
    await loadOutgoing();
    if (created?.clientRef) openUploadDialog(created.clientRef);
  } catch (e: any) {
    snackbar.text = e?.toString() || "Could not create transfer";
    snackbar.show = true;
  } finally {
    creating.value = false;
  }
}

// QR upload dialog -- same shape as expenses.vue's, minus the
// "uploaded" WS push (transfers only poll; see CheckTransferUploadStatus).
type UploadState = "waiting" | "ready" | "success" | "error";
const uploadDialog = ref(false);
const uploadState = ref<UploadState>("waiting");
const uploadUrl = ref("");
const qrDataUrl = ref("");
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
      const uploaded = await CheckTransferUploadStatus(clientRef);
      if (uploaded) markUploadSuccess();
    } catch {
      // Transient network hiccups are expected -- the next poll tick
      // will still get there.
    }
  }, UPLOAD_POLL_INTERVAL_MS);
}

function openUploadDialog(clientRef: string) {
  stopUploadWatchers();
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
    EventsOn(`transfer_upload_ready:${clientRef}`, async (url: string) => {
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
}

function closeUploadDialog() {
  uploadDialog.value = false;
}

function onUploadDialogVisibilityChange(visible: boolean) {
  if (!visible) stopUploadWatchers();
}

// ------------------------------------------------------------ INCOMING
const incomingHeaders = [
  { title: "Photo", key: "photo", sortable: false, width: 72 },
  { title: "From", key: "senderBranchName", sortable: false },
  { title: "Note", key: "note", sortable: false },
  { title: "Status", key: "status", sortable: false },
  { title: "Counted", key: "confirmed", sortable: false },
  { title: "Created", key: "createdAt", sortable: false },
];
const incoming = ref<any[]>([]);
const incomingLoading = ref(true);

function isConfirmed(item: any) {
  return (item.items ?? []).every((it: any) => it.receivedQuantity != null);
}

const unconfirmedIncomingCount = computed(
  () => incoming.value.filter((t) => t.status === "requested" && !isConfirmed(t)).length,
);

async function loadIncoming() {
  incomingLoading.value = true;
  try {
    incoming.value = await ListIncomingTransferRequests();
  } catch {
    incoming.value = [];
  }
  incomingLoading.value = false;
}

const incomingDialog = ref(false);
const incomingTarget = ref<any>(null);
const incomingItems = ref<any[]>([]);
const incomingNote = ref("");
const confirming = ref(false);

function openIncoming(item: any) {
  incomingTarget.value = item;
  incomingItems.value = (item.items ?? []).map((it: any) => ({
    ...it,
    countedQuantity: it.receivedQuantity ?? it.quantity,
  }));
  incomingNote.value = "";
  incomingDialog.value = true;
  void loadDetailPhoto("", item.id);
}

async function submitConfirm() {
  if (!incomingTarget.value) return;
  confirming.value = true;
  try {
    const items = incomingItems.value.map((it) => ({
      Barcode: it.barcode,
      ProductUID: it.productUid ?? "",
      ReceivedQuantity: it.countedQuantity,
    }));
    await ConfirmTransferRequest(incomingTarget.value.id, items, incomingNote.value);
    incomingDialog.value = false;
    await loadIncoming();
  } catch (e: any) {
    snackbar.text = e?.toString() || "Could not confirm receipt";
    snackbar.show = true;
  } finally {
    confirming.value = false;
  }
}

// ------------------------------------------------------------ LIFECYCLE
let unsubscribers: Array<() => void> = [];

onMounted(async () => {
  await Promise.all([loadOutgoing(), loadIncoming(), loadBranches()]);
  unsubscribers.push(EventsOn("sync_applied:transfer_requests", loadOutgoing));
  unsubscribers.push(EventsOn("sync_applied:incoming_transfer_requests", loadIncoming));
  unsubscribers.push(EventsOn("sync_applied:branches", loadBranches));
});

onBeforeUnmount(() => {
  unsubscribers.forEach((unsub) => unsub());
  stopUploadWatchers();
});
</script>

<style scoped>
@import "@/styles/records-page.css";
</style>
