<script setup lang="ts">
import { computed, ref } from "vue";
import { ActivateLicense } from "../../wailsjs/go/main/App";
import { WindowReloadApp } from "../../wailsjs/runtime/runtime";
import logo from "@/assets/app-logo.png";
import { activationErrorText, formatProductKey, productKeyComplete } from "@/lib/license";

// First launch on this computer: the product key ties the till to it.
const key = ref("");
const loading = ref(false);
const error = ref("");
const complete = computed(() => productKeyComplete(key.value));

function onKey(value: string) {
  key.value = formatProductKey(value);
  error.value = "";
}

async function submit() {
  if (!complete.value || loading.value) return;
  loading.value = true;
  error.value = "";
  try {
    await ActivateLicense(key.value);
    WindowReloadApp();
  } catch (e) {
    error.value = activationErrorText(e);
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="license" data-test="license-activate">
    <div class="license-card">
      <header class="license-head">
        <img :src="logo" alt="" class="license-logo" />
        <div>
          <h1 class="license-title">Activate this till</h1>
          <p class="license-sub">Enter the product key you were given. This computer needs the internet for this.</p>
        </div>
      </header>
      <k-form @submit.prevent="submit">
        <k-text-field
          :model-value="key"
          label="Product key"
          placeholder="KASHI-XXXXX-XXXXX-XXXXX-XXXXX"
          prepend-inner-icon="mdi-key-variant"
          autocomplete="off"
          spellcheck="false"
          autofocus
          hide-details
          class="license-key"
          data-test="license-key"
          @update:model-value="onKey"
        />
        <p v-if="error" class="license-error" data-test="license-error">{{ error }}</p>
        <div class="license-actions">
          <k-btn
            type="submit"
            color="primary"
            variant="flat"
            size="large"
            prepend-icon="mdi-check"
            :disabled="!complete"
            :loading="loading"
            data-test="license-submit"
          >
            Activate
          </k-btn>
        </div>
      </k-form>
    </div>
  </div>
</template>

<style scoped>
.license {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background: rgb(var(--k-theme-background));
}
.license-card {
  width: 100%;
  max-width: 520px;
  padding: 28px 28px 20px;
  background: rgb(var(--k-theme-surface));
  box-shadow: 0 18px 50px rgba(0, 0, 0, 0.18), 0 2px 6px rgba(0, 0, 0, 0.08);
}
.license-head {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 22px;
}
.license-logo {
  width: 56px;
  height: 56px;
  object-fit: contain;
  flex: 0 0 auto;
}
.license-title {
  margin: 0;
  font-size: 1.4rem;
  font-weight: 700;
  line-height: 1.2;
}
.license-sub {
  margin: 4px 0 0;
  font-size: 0.9rem;
  opacity: 0.72;
}
.license-key :deep(input) {
  font-family: ui-monospace, Menlo, Consolas, monospace;
  letter-spacing: 0.04em;
}
.license-error {
  margin: 12px 0 0;
  color: rgb(var(--k-theme-error));
  font-size: 0.9rem;
}
.license-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 18px;
}
</style>
