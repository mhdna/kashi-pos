<template>
  <v-dialog :model-value="modelValue" max-width="380" persistent @update:model-value="emit('update:modelValue', $event)">
    <v-card class="text-center pa-4">
      <template v-if="state === 'waiting'">
        <v-card-title>Preparing upload link…</v-card-title>
        <v-card-text>
          <v-progress-circular class="my-4" color="primary" indeterminate size="48" />
          <div class="text-body-2">{{ mode === "new" ? `Syncing this ${kind} to the server.` : "Asking the server for a new link." }}</div>
        </v-card-text>
      </template>

      <template v-else-if="state === 'ready'">
        <v-card-title>Attach a photo</v-card-title>
        <v-card-text>
          <div class="text-body-2">Scan this with your phone and take a photo of the receipt.</div>
          <div class="text-body-2 mb-3" dir="rtl" lang="ar">
            امسح الرمز بالهاتف والتقط صورة {{ kind === "loan" ? "الإيصال" : "سند الصرف" }}.
          </div>
          <img v-if="qrDataUrl" :src="qrDataUrl" alt="Upload QR code" style="width: 220px; height: 220px" />
          <div class="text-caption text-medium-emphasis mt-3" style="word-break: break-all">{{ uploadUrl }}</div>
        </v-card-text>
      </template>

      <template v-else-if="state === 'success'">
        <v-card-text>
          <v-icon class="my-2" color="success" icon="mdi-check-circle" size="72" />
          <div class="text-h6">Uploaded!</div>
        </v-card-text>
      </template>

      <template v-else>
        <v-card-title>Couldn't prepare the upload link</v-card-title>
        <v-card-text>
          <div class="text-body-2">{{ errorText || "Check the connection to the server and try again later." }}</div>
        </v-card-text>
      </template>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="emit('update:modelValue', false)">Close</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from "vue";
import QRCode from "qrcode";
import {
  ListExpensePhotoIDs,
  ListLoanPhotoIDs,
  RequestExpenseUploadLink,
  RequestLoanUploadLink,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

// The QR code a phone scans to upload photos for an expense or a loan.
// mode "new": the record was just saved; kashi mints the link when the
// record syncs, and it arrives as a "<kind>_upload_ready:<ref>" event.
// mode "resend": the record exists and has no photo (or room for more); the
// dialog asks kashi for a fresh link. Either way it's done when kashi pushes
// "<kind>_images_uploaded:<ref>", or, as a fallback for a dropped push, when
// polling finds more photos than there were when it opened.

const props = defineProps<{
  modelValue: boolean;
  kind: "expense" | "loan";
  clientRef: string;
  mode: "new" | "resend";
}>();
const emit = defineEmits<{
  "update:modelValue": [open: boolean];
  uploaded: [];
}>();

type State = "waiting" | "ready" | "success" | "error";
const state = ref<State>("waiting");
const uploadUrl = ref("");
const qrDataUrl = ref("");
const errorText = ref("");

const READY_TIMEOUT_MS = 30000;
const POLL_INTERVAL_MS = 4000;
let unsubscribers: Array<() => void> = [];
let pollHandle: ReturnType<typeof setInterval> | null = null;
let readyTimeout: ReturnType<typeof setTimeout> | null = null;
let session = 0;

function stopWatchers() {
  unsubscribers.forEach((unsub) => unsub());
  unsubscribers = [];
  if (pollHandle !== null) clearInterval(pollHandle);
  if (readyTimeout !== null) clearTimeout(readyTimeout);
  pollHandle = null;
  readyTimeout = null;
}

function listPhotoIDs() {
  return props.kind === "loan" ? ListLoanPhotoIDs(props.clientRef) : ListExpensePhotoIDs(props.clientRef);
}

function markSuccess() {
  if (state.value === "success") return;
  state.value = "success";
  stopWatchers();
  emit("uploaded");
}

async function showLink(url: string, mine: number) {
  if (mine !== session) return;
  if (readyTimeout !== null) clearTimeout(readyTimeout);
  readyTimeout = null;
  uploadUrl.value = url;
  try {
    qrDataUrl.value = await QRCode.toDataURL(url, { width: 220, margin: 1 });
  } catch {
    qrDataUrl.value = "";
  }
  if (mine !== session) return;
  state.value = "ready";
}

async function start() {
  stopWatchers();
  const mine = ++session;
  state.value = "waiting";
  uploadUrl.value = "";
  qrDataUrl.value = "";
  errorText.value = "";

  unsubscribers.push(EventsOn(`${props.kind}_images_uploaded:${props.clientRef}`, markSuccess));
  if (props.mode === "new") {
    // Listening before anything is awaited: sync was kicked when the
    // record was saved and the link can come back at once.
    unsubscribers.push(EventsOn(`${props.kind}_upload_ready:${props.clientRef}`, (url: string) => showLink(url, mine)));
    readyTimeout = setTimeout(() => {
      if (mine === session && state.value === "waiting") {
        state.value = "error";
        stopWatchers();
      }
    }, READY_TIMEOUT_MS);
  }
  // Photos already there when it opened don't count as this upload; a
  // record just saved has none.
  const before = props.mode === "new" ? 0 : ((await listPhotoIDs().catch(() => [])) ?? []).length;
  if (mine !== session) return;
  pollHandle = setInterval(async () => {
    if (state.value !== "ready") return;
    const now = ((await listPhotoIDs().catch(() => [])) ?? []).length;
    if (mine === session && now > before) markSuccess();
  }, POLL_INTERVAL_MS);

  if (props.mode === "resend") {
    try {
      await showLink(
        props.kind === "loan" ? await RequestLoanUploadLink(props.clientRef) : await RequestExpenseUploadLink(props.clientRef),
        mine,
      );
    } catch (e) {
      if (mine !== session) return;
      errorText.value = e instanceof Error ? e.message : String(e);
      state.value = "error";
      stopWatchers();
    }
  }
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) void start();
    else {
      session++;
      stopWatchers();
    }
  },
  { immediate: true },
);

onBeforeUnmount(() => {
  session++;
  stopWatchers();
});
</script>
