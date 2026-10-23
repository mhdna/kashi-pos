<template>
  <div class="text-center" style="width: 260px">
    <div v-if="errorText" class="text-body-2 py-6">
      {{ errorText }}
      <div class="mt-3"><v-btn color="primary" variant="tonal" @click="load">Try again</v-btn></div>
    </div>
    <template v-else>
      <img v-if="qrDataUrl" alt="Phone pairing QR code" :src="qrDataUrl" style="width: 260px; height: 260px" />
      <div v-else class="d-flex align-center justify-center" style="height: 260px">
        <v-progress-circular color="primary" indeterminate size="40" />
      </div>
      <div class="text-body-1 mt-2" :class="phoneOnline ? 'text-success' : 'text-medium-emphasis'">
        <v-icon icon="mdi-cellphone" size="20" />
        {{ phoneOnline ? "Phone connected" : "Scan once with the phone, keep the page open" }}
      </div>
      <div v-if="!phoneOnline" class="text-body-2 text-medium-emphasis" dir="rtl" lang="ar">
        امسح الرمز مرة واحدة بالهاتف وأبقِ الصفحة مفتوحة
      </div>
      <v-btn class="mt-1" size="small" variant="text" :loading="resetting" @click="pairAnother">Pair another phone</v-btn>
    </template>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import QRCode from "qrcode";
import { GetUnknownPairing, ResetUnknownPairing } from "../../wailsjs/go/main/App";

// The QR code of this till's paired phone (see unknown_items.go): the same
// link every time, so a phone scans it once and keeps the page open. Shows
// live whether that phone is connected.

const emit = defineEmits<{ online: [online: boolean] }>();

const REFRESH_MS = 2000;
const qrDataUrl = ref("");
const phoneOnline = ref(false);
const errorText = ref("");
const resetting = ref(false);
let url = "";
let timer: ReturnType<typeof setTimeout> | undefined;
let alive = true;

async function show(info: Record<string, any>) {
  phoneOnline.value = Boolean(info.phoneOnline);
  emit("online", phoneOnline.value);
  if (info.url && info.url !== url) {
    url = String(info.url);
    qrDataUrl.value = await QRCode.toDataURL(url, { width: 260, margin: 1 });
  }
}

async function load() {
  clearTimeout(timer);
  errorText.value = "";
  try {
    await show(await GetUnknownPairing());
  } catch (e) {
    if (!url) errorText.value = e instanceof Error ? e.message : String(e);
  }
  if (alive && !errorText.value) timer = setTimeout(load, REFRESH_MS);
}

async function pairAnother() {
  resetting.value = true;
  try {
    await show(await ResetUnknownPairing());
  } catch (e) {
    errorText.value = e instanceof Error ? e.message : String(e);
  } finally {
    resetting.value = false;
  }
}

onMounted(load);
onBeforeUnmount(() => {
  alive = false;
  clearTimeout(timer);
});
</script>
