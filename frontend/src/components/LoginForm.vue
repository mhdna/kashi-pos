<template>
  <div class="pin-login" aria-labelledby="pin-title">
    <v-img
      :src="logo"
      width="132"
      class="pin-logo"
      :class="{ 'pin-logo--dark': currentTheme === 'dark' }"
      alt="Kashi"
    />

    <div class="pin-heading">
      <h1 id="pin-title">Enter PIN</h1>
      <p v-if="locked">Too many attempts. Try again in {{ lockSeconds }}s</p>
    </div>

    <div class="pin-dots" :class="{ 'pin-dots--error': error }" aria-label="PIN entry">
      <span v-for="index in 4" :key="index" :class="{ filled: index <= pin.length }" />
    </div>

    <div class="pin-keypad" :aria-disabled="loading || locked">
      <v-btn
        v-for="digit in digits"
        :key="digit"
        type="button"
        class="pin-key"
        variant="tonal"
        rounded="0"
        elevation="0"
        :disabled="loading || locked"
        :aria-label="`Enter ${digit}`"
        @click="enterDigit(digit)"
      >
        {{ digit }}
      </v-btn>
      <v-btn
        type="button"
        class="pin-key pin-key--action"
        variant="text"
        rounded="0"
        :disabled="loading || locked || pin.length === 0"
        aria-label="Clear PIN"
        @click="clearPIN"
      >
        Clear
      </v-btn>
      <v-btn
        type="button"
        class="pin-key"
        variant="tonal"
        rounded="0"
        elevation="0"
        :disabled="loading || locked"
        aria-label="Enter 0"
        @click="enterDigit('0')"
      >
        0
      </v-btn>
      <v-btn
        type="button"
        class="pin-key pin-key--action"
        variant="text"
        rounded="0"
        :disabled="loading || locked || pin.length === 0"
        aria-label="Delete last digit"
        @click="removeDigit"
      >
        <v-icon icon="mdi-backspace-outline" size="24" />
      </v-btn>
    </div>

    <div class="pin-status" aria-live="polite">
      <v-progress-circular v-if="loading" indeterminate size="22" width="2" />
      <span v-else-if="!locked && error" class="pin-error" role="alert">{{ error }}</span>
      <span v-if="!locked && attempts > 0">
        {{ 3 - attempts }} {{ 3 - attempts === 1 ? "attempt" : "attempts" }} remaining
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useTheme } from "vuetify";
import logo from "@/assets/appicon.webp";
import { useAuthStore } from "@/stores/auth";

const LOCKOUT_SECONDS = 30;
const digits = ["1", "2", "3", "4", "5", "6", "7", "8", "9"];

const router = useRouter();
const auth = useAuthStore();
const theme = useTheme();
const currentTheme = theme.name;

const pin = ref("");
const error = ref("");
const loading = ref(false);
const attempts = ref(0);
const lockSeconds = ref(0);
const locked = computed(() => lockSeconds.value > 0);
let lockTimer: ReturnType<typeof setInterval> | undefined;

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
  const enteredPIN = pin.value;
  try {
    const result = await auth.loginWithPIN(enteredPIN);
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

function handleKeydown(event: KeyboardEvent) {
  if (/^\d$/.test(event.key)) enterDigit(event.key);
  else if (event.key === "Backspace") removeDigit();
  else if (event.key === "Escape" || event.key === "Delete") clearPIN();
}

onMounted(() => window.addEventListener("keydown", handleKeydown));
onBeforeUnmount(() => {
  window.removeEventListener("keydown", handleKeydown);
  if (lockTimer) clearInterval(lockTimer);
});
</script>

<style scoped>
.pin-login {
  width: min(100%, 380px);
  padding: 32px 34px 26px;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.pin-logo {
  margin-bottom: 24px;
}

.pin-logo--dark {
  filter: invert(1);
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
  color: rgba(var(--v-theme-on-surface), 0.62);
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
  border: 2px solid rgba(var(--v-theme-on-surface), 0.32);
  border-radius: 50%;
  transition: background-color 120ms ease, border-color 120ms ease, transform 120ms ease;
}

.pin-dots span.filled {
  border-color: rgb(var(--v-theme-on-surface));
  background: rgb(var(--v-theme-on-surface));
  transform: scale(1.08);
}

.pin-dots--error span {
  border-color: rgb(var(--v-theme-error));
}

.pin-keypad {
  display: grid;
  grid-template-columns: repeat(3, 64px);
  gap: 10px;
}

.pin-key {
  width: 64px;
  height: 52px;
  border: 1px solid rgba(var(--v-theme-on-surface), 0.12);
  border-radius: 0 !important;
  color: rgb(var(--v-theme-on-surface));
  font: inherit;
  font-size: 1.45rem;
  font-weight: 600;
  cursor: pointer;
  touch-action: manipulation;
}

.pin-key:hover:not(:disabled),
.pin-key:focus-visible {
  border-color: rgba(var(--v-theme-on-surface), 0.25);
  background: rgba(var(--v-theme-on-surface), 0.1);
  outline: none;
}

.pin-key:active:not(:disabled) {
  transform: scale(0.97);
}

.pin-key:disabled {
  cursor: default;
  opacity: 0.42;
}

.pin-key--action {
  border-color: transparent !important;
  font-size: 0.78rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.pin-status {
  min-height: 24px;
  margin-top: 18px;
  color: rgba(var(--v-theme-on-surface), 0.55);
  font-size: 0.8rem;
}

.pin-status .pin-error {
  display: block;
  margin-bottom: 4px;
  color: rgb(var(--v-theme-error));
}

@media (max-width: 440px), (max-height: 700px) {
  .pin-login {
    width: 100%;
    padding: 22px 24px 16px;
  }

  .pin-logo {
    width: 108px !important;
    margin-bottom: 14px;
  }

  .pin-dots {
    margin: 10px 0 18px;
  }

  .pin-keypad {
    grid-template-columns: repeat(3, 58px);
    gap: 8px;
  }

  .pin-key {
    width: 58px;
    height: 46px;
  }
}
</style>
