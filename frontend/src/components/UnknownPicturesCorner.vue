<template>
  <k-menu location="top" :close-on-content-click="false">
    <template #activator="{ props: menuProps }">
      <k-card
        v-bind="menuProps"
        class="unknown-corner d-flex align-center ga-3 pa-2 mx-3 mb-2"
        :class="{ 'unknown-corner--done': complete }"
        rounded="lg"
        variant="outlined"
      >
        <img v-if="qrDataUrl" alt="Unknown item pictures QR code" :src="qrDataUrl" class="unknown-corner__qr" />
        <div v-else class="unknown-corner__qr d-flex align-center justify-center">
          <k-progress-circular v-if="!errorText" indeterminate size="28" />
          <k-icon v-else color="warning" icon="mdi-alert-circle-outline" size="32" />
        </div>
        <div class="flex-grow-1" style="min-width: 0; line-height: 1.35">
          <div class="text-subtitle-1 font-weight-bold">Unknown items need a picture</div>
          <div class="text-body-2">Scan this code with the phone, then photograph each unknown item.</div>
          <div class="text-body-2 text-medium-emphasis" dir="rtl" lang="ar">
            امسح الرمز والتقط صورة لكل قطعة غير معروف كودها طبقا للسعر
          </div>
          <div class="d-flex align-center ga-1 mt-1 text-body-2 font-weight-medium" :class="complete ? 'text-success' : 'text-medium-emphasis'">
            <k-icon :icon="complete ? 'mdi-check-circle' : 'mdi-camera-outline'" size="18" />
            {{ errorText ? "No QR code yet, retrying…" : `${doneCount} of ${lines.length} pictured` }}
          </div>
        </div>
      </k-card>
    </template>
    <k-card class="pa-4 text-center" max-width="320">
      <img v-if="qrDataUrl" alt="Unknown item pictures QR code" :src="qrDataUrl" style="width: 260px; height: 260px" />
      <div class="text-body-1 mt-2">Scan, then photograph each item.</div>
      <div class="text-body-2 text-medium-emphasis" dir="rtl" lang="ar">
        امسح الرمز والتقط صورة لكل قطعة غير معروف كودها طبقا للسعر
      </div>
      <div v-if="errorText" class="text-caption text-warning mt-2">{{ errorText }}</div>
    </k-card>
  </k-menu>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import QRCode from "qrcode";
import { SyncUnknownItemSession, UnknownItemSessionStatus } from "../../wailsjs/go/main/App";

// The sale screen's QR code for the unknown items' pictures, a strip under
// the cart (where Quick Access sits) shown as soon as one is in the cart (unless the branch takes them at checkout). It
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
  border-color: rgb(var(--k-theme-success));
}
.unknown-corner__qr {
  width: 112px;
  height: 112px;
  flex: none;
  image-rendering: pixelated;
}
</style>
