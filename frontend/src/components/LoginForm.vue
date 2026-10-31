<template>
  <div class="pin-login" aria-labelledby="pin-title">
    <div class="pin-logo-wrap">
      <div
        ref="logoEl"
        class="pin-logo"
        :style="{ '--pin-logo-image': `url(${logo})` }"
        role="img"
        aria-label="K"
      />
      <k-btn
        class="pin-admin-toggle"
        :icon="mode === 'admin' ? 'mdi-keyboard-outline' : 'mdi-shield-key-outline'"
        size="small"
        variant="text"
        :aria-label="mode === 'admin' ? 'Back to cashier PIN' : 'Admin sign-in'"
        :title="mode === 'admin' ? 'Back to cashier PIN' : 'Admin sign-in'"
        @click="toggleMode"
      />
    </div>

    <k-select
      v-if="branches.length > 1"
      class="pin-branch-select"
      :model-value="activeBranchCode"
      density="compact"
      hide-details
      item-title="name"
      item-value="code"
      :items="branches"
      :loading="switchingBranch"
      :disabled="switchingBranch"
      prepend-inner-icon="mdi-store-outline"
      variant="outlined"
      @update:model-value="switchBranch"
    />
    <span v-if="branchError" class="pin-branch-error" role="alert">{{ branchError }}</span>

    <div class="pin-heading">
      <h1 id="pin-title">{{ mode === "admin" ? "Admin sign-in" : "Enter PIN" }}</h1>
      <p v-if="locked">Too many attempts. Try again in {{ lockSeconds }}s</p>
    </div>

    <template v-if="mode === 'pin'">
      <div class="pin-dots" :class="{ 'pin-dots--error': error }" aria-label="PIN entry">
        <span v-for="index in 4" :key="index" :class="{ filled: index <= pin.length }" />
      </div>

      <div class="pin-keypad" :aria-disabled="loading || locked">
        <k-btn
          v-for="digit in digits"
          :key="digit"
          type="button"
          class="pin-key"
          size="small"
          variant="tonal"
          :disabled="loading || locked"
          :aria-label="`Enter ${digit}`"
          @click="enterDigit(digit)"
        >
          {{ digit }}
        </k-btn>
        <k-btn
          type="button"
          class="pin-key"
          size="small"
          variant="tonal"
          :disabled="loading || locked || pin.length === 0"
          aria-label="Clear PIN"
          @click="clearPIN"
        >
          C
        </k-btn>
        <k-btn
          type="button"
          class="pin-key"
          size="small"
          variant="tonal"
          :disabled="loading || locked"
          aria-label="Enter 0"
          @click="enterDigit('0')"
        >
          0
        </k-btn>
        <k-btn
          type="button"
          class="pin-key"
          size="small"
          variant="tonal"
          :disabled="loading || locked || pin.length === 0"
          aria-label="Delete last digit"
          @click="removeDigit"
        >
          ⌫
        </k-btn>
      </div>
    </template>

    <form v-else class="pin-admin-form" @submit.prevent="submitAdmin">
      <k-text-field
        v-model="adminPassword"
        autofocus
        type="password"
        placeholder="Admin password"
        variant="outlined"
        density="comfortable"
        hide-details
        :disabled="loading || locked"
        :class="{ 'pin-admin-field--error': error }"
      />
      <k-btn
        type="submit"
        block
        color="primary"
        size="large"
        class="mt-3"
        :loading="loading"
        :disabled="locked || adminPassword.length === 0"
      >
        Sign in
      </k-btn>
    </form>

    <div class="pin-status" aria-live="polite">
      <k-progress-circular v-if="loading && mode === 'pin'" indeterminate size="22" width="2" />
      <span v-else-if="!locked && error" class="pin-error" role="alert">{{ error }}</span>
      <span v-if="mode === 'pin' && !locked && attempts > 0" class="pin-attempts text-subtitle-1">
        {{ 3 - attempts }} {{ 3 - attempts === 1 ? "attempt" : "attempts" }} remaining
      </span>
    </div>
    <div v-if="appVersion" class="pin-version">{{ appVersion }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import logo from "@/assets/app-logo.png";
import { useAuthStore } from "@/stores/auth";
import { GetAppVersion, ListBranches, SwitchBranch } from "../../wailsjs/go/main/App";
import { WindowReloadApp } from "../../wailsjs/runtime/runtime";

const LOCKOUT_SECONDS = 30;
const digits = ["1", "2", "3", "4", "5", "6", "7", "8", "9"];

const router = useRouter();
const auth = useAuthStore();
const mode = ref<"pin" | "admin">("pin");
const pin = ref("");
const adminPassword = ref("");
const error = ref("");
const loading = ref(false);
const attempts = ref(0);
const lockSeconds = ref(0);
const locked = computed(() => lockSeconds.value > 0);
const appVersion = ref("");
let lockTimer: ReturnType<typeof setInterval> | undefined;

const branches = ref<{ code: string; name: string }[]>([]);
const activeBranchCode = ref("");
const switchingBranch = ref(false);
const branchError = ref("");

async function switchBranch(code: string) {
  if (!code || code === activeBranchCode.value || switchingBranch.value) return;
  switchingBranch.value = true;
  branchError.value = "";
  try {
    await SwitchBranch(code);
    // The active branch's database is now a different one than this window
    // booted with (settings, users, PIN/admin password all live in it), so
    // a full reload -- not just re-fetching state -- is required; see
    // SwitchBranch's own doc comment and App.vue's admin switcher, which
    // does the same.
    WindowReloadApp();
  } catch (error) {
    branchError.value = error instanceof Error ? error.message : String(error);
    switchingBranch.value = false;
  }
}

function toggleMode() {
  if (loading.value) return;
  mode.value = mode.value === "pin" ? "admin" : "pin";
  error.value = "";
  pin.value = "";
  adminPassword.value = "";
}

function enterDigit(digit: string) {
  if (loading.value || locked.value || pin.value.length >= 4) return;
  error.value = "";
  pin.value += digit;
  if (pin.value.length === 4) void submitPIN();
}

function removeDigit() {
  if (loading.value || locked.value) return;
  error.value = "";
  pin.value = pin.value.slice(0, -1);
}

function clearPIN() {
  if (loading.value || locked.value) return;
  error.value = "";
  pin.value = "";
}

function startLockout(seconds = LOCKOUT_SECONDS) {
  attempts.value = 0;
  lockSeconds.value = Math.max(1, seconds);
  if (lockTimer) clearInterval(lockTimer);
  lockTimer = setInterval(() => {
    lockSeconds.value--;
    if (lockSeconds.value <= 0) {
      clearInterval(lockTimer);
      lockTimer = undefined;
      error.value = "";
    }
  }, 1000);
}

async function submitPIN() {
  if (loading.value || locked.value || pin.value.length !== 4) return;
  loading.value = true;
  try {
    const result = await auth.loginWithPIN(pin.value);
    if (result.ok) {
      attempts.value = 0;
      await router.replace("/home");
      return;
    }

    pin.value = "";
    if (result.retryAfter > 0) {
      error.value = "";
      startLockout(result.retryAfter);
      return;
    }

    attempts.value++;
    error.value = "Incorrect PIN";
    if (attempts.value >= 3) startLockout();
  } finally {
    loading.value = false;
  }
}

async function submitAdmin() {
  if (loading.value || locked.value || adminPassword.value.length === 0) return;
  loading.value = true;
  try {
    const result = await auth.loginWithAdminPassword(adminPassword.value);
    if (result.ok) {
      adminPassword.value = "";
      await router.replace("/home");
      return;
    }

    adminPassword.value = "";
    if (result.retryAfter > 0) {
      error.value = "";
      startLockout(result.retryAfter);
      return;
    }
    error.value = "Incorrect password";
  } finally {
    loading.value = false;
  }
}

function handleKeydown(event: KeyboardEvent) {
  if (mode.value !== "pin") return;
  if (/^\d$/.test(event.key)) enterDigit(event.key);
  else if (event.key === "Backspace") removeDigit();
  else if (event.key === "Escape" || event.key === "Delete") clearPIN();
}

// The startup loader in index.html draws this same logo, and puts it where
// this one sits so the hand-off to the login screen doesn't move it. The
// spot is kept as an offset from the window centre, plus the colours, for
// the loader to read on the next launch.
const logoEl = ref<HTMLElement | null>(null);
function rememberLogoSpot() {
  const el = logoEl.value;
  if (!el) return;
  const rect = el.getBoundingClientRect();
  if (!rect.width) return;
  const app = el.closest(".k-app");
  try {
    localStorage.setItem(
      "startupLogoSpot",
      JSON.stringify({
        dx: Math.round(rect.left + rect.width / 2 - window.innerWidth / 2),
        dy: Math.round(rect.top + rect.height / 2 - window.innerHeight / 2),
        size: Math.round(rect.width),
        color: getComputedStyle(el).backgroundColor,
        background: app ? getComputedStyle(app).backgroundColor : "",
      }),
    );
  } catch {
    // Storage unavailable: the loader falls back to a centred logo.
  }
}

onMounted(async () => {
  window.addEventListener("keydown", handleKeydown);
  window.addEventListener("resize", rememberLogoSpot);
  const status = await auth.getPINLockoutStatus();
  if (status.locked) startLockout(status.retryAfter);
  try {
    appVersion.value = await GetAppVersion();
  } catch {
    appVersion.value = "";
  }
  try {
    const result = await ListBranches();
    branches.value = Array.isArray(result?.branches) ? result.branches : [];
    activeBranchCode.value = result?.active ?? "";
  } catch {
    branches.value = [];
  }
  // After the branch list and version have rendered: they change the
  // form's height, which moves the centred logo.
  await nextTick();
  rememberLogoSpot();
});
onBeforeUnmount(() => {
  window.removeEventListener("keydown", handleKeydown);
  window.removeEventListener("resize", rememberLogoSpot);
  if (lockTimer) clearInterval(lockTimer);
});
</script>

<style scoped>
.pin-login {
  width: min(100%, 380px);
  padding: 32px 34px;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.pin-logo-wrap {
  position: relative;
  display: flex;
  justify-content: center;
}

.pin-logo {
  aspect-ratio: 1;
  width: clamp(105px, 14vw, 190px);
  background-color: rgb(var(--k-theme-on-surface));
  mask: var(--pin-logo-image) center / contain no-repeat;
  -webkit-mask: var(--pin-logo-image) center / contain no-repeat;
  margin-bottom: 0;
}

.pin-admin-toggle {
  position: absolute;
  right: -8px;
  bottom: -4px;
  opacity: 0.55;
}

.pin-admin-toggle:hover,
.pin-admin-toggle:focus-visible {
  opacity: 1;
}

.pin-branch-select {
  width: 280px;
  margin-top: 18px;
}

.pin-branch-error {
  margin-top: 4px;
  color: rgb(var(--k-theme-error));
  font-size: 0.8rem;
}

.pin-heading {
  min-height: 40px;
  text-align: center;
}

.pin-heading h1 {
  font-size: 1.25rem;
  line-height: 1.2;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.pin-heading p {
  margin-top: 8px;
  color: rgba(var(--k-theme-on-surface), 0.62);
  font-size: 0.9rem;
}

.pin-dots {
  height: 24px;
  margin: 18px 0 28px;
  display: flex;
  align-items: center;
  gap: 18px;
}

.pin-dots span {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(var(--k-theme-on-surface), 0.32);
  border-radius: 50%;
  transition: background-color 120ms ease, border-color 120ms ease, transform 120ms ease;
}

.pin-dots span.filled {
  border-color: rgb(var(--k-theme-on-surface));
  background: rgb(var(--k-theme-on-surface));
  transform: scale(1.08);
}

.pin-dots--error span {
  border-color: rgb(var(--k-theme-error));
}

.pin-keypad {
  width: 280px;
  display: grid;
  grid-auto-rows: 56px;
  grid-template-columns: repeat(3, 1fr);
  gap: 6px;
}

.pin-key {
  border: 1px solid rgba(var(--k-border-color), 0.18);
  border-radius: 4px;
  height: 100%;
  min-width: 0;
  padding: 0;
  font: inherit;
  font-size: 2rem !important;
  cursor: pointer;
  touch-action: manipulation;
}

.pin-key:hover:not(:disabled),
.pin-key:focus-visible {
  border-color: currentColor;
  outline: none;
}

.pin-key:active:not(:disabled) {
  transform: scale(0.97);
}

.pin-key:disabled {
  cursor: default;
  opacity: 0.42;
}

.pin-admin-form {
  width: 280px;
  margin: 18px 0 4px;
}

.pin-admin-field--error :deep(.k-field) {
  border-color: rgb(var(--k-theme-error));
}

.pin-status {
  height: 44px;
  margin-top: 18px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: flex-start;
  color: rgba(var(--k-theme-on-surface), 0.55);
  font-size: 0.8rem;
}

.pin-status .pin-error {
  display: block;
  margin-bottom: 4px;
  color: rgb(var(--k-theme-error));
  font-size: 1rem;
  font-weight: 600;
}

.pin-version {
  margin-top: 4px;
  color: rgba(var(--k-theme-on-surface), 0.4);
  font-size: 0.7rem;
  text-align: center;
}

@media (max-width: 440px), (max-height: 700px) {
  .pin-login {
    width: 100%;
    padding: 22px 24px;
  }

  .pin-logo {
    width: clamp(105px, 14vw, 190px) !important;
    margin-bottom: 0;
  }

  .pin-dots {
    margin: 10px 0 18px;
  }

  .pin-keypad {
    width: 100%;
    max-width: 280px;
  }

  .pin-admin-form {
    width: 100%;
    max-width: 280px;
  }
}
</style>
