<template>
  <v-menu location="bottom" :close-on-content-click="false">
    <template #activator="{ props: menuProps }">
      <v-card
        v-bind="menuProps"
        class="unknown-corner d-flex align-center ga-2 pa-1 pe-3"
        :class="{ 'unknown-corner--done': complete }"
        rounded="lg"
        variant="outlined"
      >
        <img v-if="qrDataUrl" alt="Unknown item pictures QR code" :src="qrDataUrl" class="unknown-corner__qr" />
        <div v-else class="unknown-corner__qr d-flex align-center justify-center">
          <v-progress-circular v-if="!errorText" indeterminate size="22" />
          <v-icon v-else color="warning" icon="mdi-alert-circle-outline" />
        </div>
        <div class="text-body-2" style="line-height: 1.25">
          <div class="font-weight-medium">{{ errorText ? "No QR yet" : "Unknown items" }}</div>
          <div class="d-flex align-center ga-1 text-medium-emphasis">
            <v-icon v-if="complete" color="success" icon="mdi-check-circle" size="16" />
            {{ errorText ? "Retrying…" : `${doneCount} of ${lines.length} pictured` }}
          </div>
        </div>
      </v-card>
    </template>
    <v-card class="pa-4 text-center" max-width="320">
      <img v-if="qrDataUrl" alt="Unknown item pictures QR code" :src="qrDataUrl" style="width: 260px; height: 260px" />
      <div class="text-body-1 mt-2">Scan, then photograph each item.</div>
      <div class="text-body-2 text-medium-emphasis" dir="rtl" lang="ar">
        امسح الرمز والتقط صورة لكل قطعة غير معروف كودها طبقا للسعر
      </div>
      <div v-if="errorText" class="text-caption text-warning mt-2">{{ errorText }}</div>
    </v-card>
  </v-menu>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import QRCode from "qrcode";
import { SyncUnknownItemSession, UnknownItemSessionStatus } from "../../wailsjs/go/main/App";

// The sale screen's small QR code for the unknown items' pictures, shown as
// soon as one is in the cart (unless the branch takes them at checkout). It
// keeps one kashi session per cart and re-sends it whenever the unknown items
// change, so the phone already on the link just sees the new items appear.
// It never blocks the cashier: paying still goes through UnknownPicturesDialog,
// which skips itself once this has every picture.

type Unit = { label: string; unitPriceCents: number; discount: number; lineRef: string; slotRef: string };
type Slot = { slotRef: string; lineRef?: string; hasImage: boolean; waived: boolean; done: boolean };

const props = defineProps<{ units: Unit[]; clientRef: string; paused?: boolean }>();
const emit = defineEmits<{
  // The session opened for this cart (kept by the page for the next sync).
  session: [clientRef: string];
  // Every unit of the cart `signature` describes has a picture or a pass.
  ready: [signature: string];
}>();

const SYNC_DELAY_MS = 300;
const RETRY_MS = 3000;
const slots = ref<Slot[]>([]);
const complete = ref(false);
const qrDataUrl = ref("");
const errorText = ref("");
let uploadUrl = "";
// Bumped by every sync and on pause/unmount, so a stale sync or poll drops
// its answer.
let generation = 0;
let syncTimer: ReturnType<typeof setTimeout> | undefined;

const lines = computed(() => {
  const byKey = new Map<string, boolean>();
  for (const slot of slots.value) {
    const key = slot.lineRef || `unit:${slot.slotRef}`;
    byKey.set(key, (byKey.get(key) ?? true) && slot.done);
  }
  return [...byKey.values()];
});
const doneCount = computed(() => lines.value.filter(Boolean).length);
const signature = computed(() => JSON.stringify(props.units));

function apply(status: any, sig: string) {
  slots.value = status.slots ?? [];
  complete.value = Boolean(status.complete);
  if (complete.value) emit("ready", sig);
}

async function pollLoop(mine: number, clientRef: string, sig: string) {
  while (mine === generation) {
    try {
      const status = await UnknownItemSessionStatus(clientRef);
      if (mine !== generation) return;
      apply(status, sig);
    } catch {
      await new Promise((resolve) => setTimeout(resolve, RETRY_MS));
    }
  }
}

async function sync() {
  const mine = ++generation;
  const sig = signature.value;
  if (!props.units.length) return;
  try {
    const info = await SyncUnknownItemSession(props.clientRef, sig);
    if (mine !== generation) return;
    errorText.value = "";
    if (info.clientRef !== props.clientRef) emit("session", info.clientRef);
    if (info.uploadUrl !== uploadUrl) {
      uploadUrl = info.uploadUrl;
      qrDataUrl.value = await QRCode.toDataURL(uploadUrl, { width: 260, margin: 1 });
      if (mine !== generation) return;
    }
    apply(info, sig);
    void pollLoop(mine, info.clientRef, sig);
  } catch (e) {
    if (mine !== generation) return;
    errorText.value = e instanceof Error ? e.message : String(e);
    syncTimer = setTimeout(() => {
      if (mine === generation) void sync();
    }, RETRY_MS);
  }
}

function scheduleSync() {
  clearTimeout(syncTimer);
  generation++;
  if (props.paused) return;
  syncTimer = setTimeout(() => void sync(), SYNC_DELAY_MS);
}

// A new cart (the page cleared clientRef) gets a new link.
watch(
  () => props.clientRef,
  (ref) => {
    if (!ref) {
      uploadUrl = "";
      qrDataUrl.value = "";
    }
  },
);
watch([signature, () => props.paused], scheduleSync, { immediate: true });
onBeforeUnmount(() => {
  clearTimeout(syncTimer);
  generation++;
});
</script>

<style scoped>
.unknown-corner {
  cursor: pointer;
  user-select: none;
}
.unknown-corner--done {
  border-color: rgb(var(--v-theme-success));
}
.unknown-corner__qr {
  width: 88px;
  height: 88px;
  flex: none;
  image-rendering: pixelated;
}
</style>
