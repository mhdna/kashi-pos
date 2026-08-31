<template>
  <div class="pin-login" aria-labelledby="pin-title">
    <div
      class="pin-logo"
      :style="{ '--pin-logo-image': `url(${logo})` }"
      role="img"
      aria-label="K"
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
        size="small"
        variant="tonal"
        :disabled="loading || locked"
        :aria-label="`Enter ${digit}`"
        @click="enterDigit(digit)"
      >
        {{ digit }}
      </v-btn>
      <v-btn
        type="button"
        class="pin-key"
        size="small"
        variant="tonal"
        :disabled="loading || locked || pin.length === 0"
        aria-label="Clear PIN"
        @click="clearPIN"
      >
        C
      </v-btn>
      <v-btn
        type="button"
        class="pin-key"
        size="small"
        variant="tonal"
        :disabled="loading || locked"
        aria-label="Enter 0"
        @click="enterDigit('0')"
      >
        0
      </v-btn>
      <v-btn
        type="button"
        class="pin-key"
        size="small"
        variant="tonal"
        :disabled="loading || locked || pin.length === 0"
        aria-label="Delete last digit"
        @click="removeDigit"
      >
        ⌫
      </v-btn>
    </div>

    <div class="pin-status" aria-live="polite">
      <v-progress-circular v-if="loading" indeterminate size="22" width="2" />
      <span v-else-if="!locked && error" class="pin-error" role="alert">{{ error }}</span>
      <span v-if="!locked && attempts > 0" class="pin-attempts text-subtitle-1">
        {{ 3 - attempts }} {{ 3 - attempts === 1 ? "attempt" : "attempts" }} remaining
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import logo from "@/assets/app-logo.png";
import { useAuthStore } from "@/stores/auth";

const LOCKOUT_SECONDS = 30;
const digits = ["1", "2", "3", "4", "5", "6", "7", "8", "9"];

const router = useRouter();
const auth = useAuthStore();
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

onMounted(async () => {
  window.addEventListener("keydown", handleKeydown);
  const status = await auth.getPINLockoutStatus();
  if (status.locked) startLockout(status.retryAfter);
});
onBeforeUnmount(() => {
  window.removeEventListener("keydown", handleKeydown);
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

.pin-logo {
  aspect-ratio: 1;
  width: clamp(105px, 14vw, 190px);
  background-color: rgba(var(--v-theme-on-surface), 0.38);
  mask: var(--pin-logo-image) center / contain no-repeat;
  -webkit-mask: var(--pin-logo-image) center / contain no-repeat;
  margin-bottom: 0;
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
  width: 280px;
  display: grid;
  grid-auto-rows: 56px;
  grid-template-columns: repeat(3, 1fr);
  gap: 6px;
}

.pin-key {
  border: 1px solid rgba(var(--v-border-color), 0.18);
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

.pin-status {
  height: 44px;
  margin-top: 18px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: flex-start;
  color: rgba(var(--v-theme-on-surface), 0.55);
  font-size: 0.8rem;
}

.pin-status .pin-error {
  display: block;
  margin-bottom: 4px;
  color: rgb(var(--v-theme-error));
  font-size: 1rem;
  font-weight: 600;
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
}
</style>
