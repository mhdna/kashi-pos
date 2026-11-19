<script setup lang="ts">
import { ref } from "vue";
import { RetryLicense } from "../../wailsjs/go/main/App";
import { WindowReloadApp } from "../../wailsjs/runtime/runtime";
import { LOCK_MESSAGE } from "@/lib/license";

// Shown over everything when the till may not run. It never says why; the
// code is for a support call.
defineProps<{ code?: string }>();
const loading = ref(false);

async function retry() {
  loading.value = true;
  try {
    const status = await RetryLicense();
    if (status?.state === "ok") WindowReloadApp();
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="lock" data-test="license-lock">
    <div class="lock-box">
      <k-icon icon="mdi-alert-circle-outline" size="40" class="lock-icon" />
      <p class="lock-message" data-test="license-lock-message">{{ LOCK_MESSAGE }}</p>
      <p v-if="code" class="lock-code" data-test="license-lock-code">Code {{ code }}</p>
      <k-btn variant="outlined" prepend-icon="mdi-refresh" :loading="loading" data-test="license-retry" @click="retry">
        Try again
      </k-btn>
    </div>
  </div>
</template>

<style scoped>
.lock {
  position: fixed;
  inset: 0;
  z-index: 10000;
  display: grid;
  place-items: center;
  padding: 24px;
  background: rgb(var(--k-theme-background));
}
.lock-box {
  max-width: 440px;
  text-align: center;
}
.lock-icon {
  color: rgb(var(--k-theme-error));
}
.lock-message {
  margin: 14px 0 6px;
  font-size: 1.15rem;
  font-weight: 600;
}
.lock-code {
  margin: 0 0 18px;
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: 0.85rem;
  opacity: 0.6;
}
</style>
