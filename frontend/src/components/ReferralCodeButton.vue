<template>
  <div v-if="enabled" class="header-btn-wrap">
    <div class="header-btn-subtitle">Referral</div>
    <k-btn
      size="small"
      variant="text"
      aria-label="Referral code"
      class="referral-btn"
      :class="{ 'referral-has-code': Boolean(modelValue) }"
      data-test="referral-open"
      @click="open"
    >
      <k-icon :icon="modelValue ? 'mdi-check-circle' : 'mdi-gift-outline'" size="20" class="flex-shrink-0" />
    </k-btn>
    <k-dialog v-model="dialog" max-width="560">
      <k-card>
        <k-card-title>Referral code</k-card-title>
        <k-card-text>
          <p class="referral-explain">
            A new customer can give the code of the client who sent them. That client gets
            {{ rewardPercent }}% off their next visit.
          </p>
          <k-text-field
            v-model="draft"
            autofocus
            hide-details
            placeholder="Code, like K7M4QH"
            variant="outlined"
            data-test="referral-input"
            @keydown.enter="check"
          />
          <div v-if="message" class="referral-message" :class="messageError ? 'text-error' : ''" data-test="referral-message">
            {{ message }}
          </div>
          <InlineKeyboard v-model="draft" :max-length="12" class="mt-3" />
        </k-card-text>
        <k-card-actions class="pa-4">
          <k-btn v-if="modelValue" variant="text" color="error" data-test="referral-remove" @click="remove">Remove</k-btn>
          <k-spacer />
          <k-btn variant="text" @click="dialog = false">Cancel</k-btn>
          <k-btn color="primary" variant="elevated" :loading="checking" data-test="referral-add" @click="check">Add</k-btn>
        </k-card-actions>
      </k-card>
    </k-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import InlineKeyboard from "@/components/InlineKeyboard.vue";
import { CheckReferralCode, GetReferralInfo } from "../../wailsjs/go/main/App";
import { normalizeReferralCode, referralCodeHint } from "@/lib/referralCode";

const props = defineProps<{ modelValue: string; clientId: number }>();
const emit = defineEmits<{ "update:modelValue": [code: string] }>();

const enabled = ref(false);
const rewardPercent = ref(0);
const dialog = ref(false);
const draft = ref("");
const checking = ref(false);
const serverError = ref("");
const ownerName = ref("");

async function loadInfo() {
  try {
    const info = await GetReferralInfo(0);
    enabled.value = Boolean(info.enabled);
    rewardPercent.value = Number(info.rewardPercent);
  } catch {
    enabled.value = false;
  }
}
void loadInfo();

const hint = computed(() => referralCodeHint(draft.value));
const message = computed(() => {
  if (serverError.value) return serverError.value;
  if (hint.value) return hint.value;
  if (props.modelValue && ownerName.value && normalizeReferralCode(draft.value) === props.modelValue) {
    return `Sent by ${ownerName.value}`;
  }
  return "";
});
const messageError = computed(() => Boolean(serverError.value) || /typo/.test(hint.value));

watch(draft, () => {
  serverError.value = "";
});

// Whether a code counts depends on the customer: another one starts over.
watch(
  () => props.clientId,
  () => {
    if (props.modelValue) emit("update:modelValue", "");
    ownerName.value = "";
  },
);

function open() {
  void loadInfo();
  draft.value = props.modelValue;
  serverError.value = "";
  dialog.value = true;
}

function errorText(e: unknown): string {
  return typeof e === "string" ? e : e instanceof Error ? e.message : String(e);
}

async function check() {
  if (hint.value) {
    serverError.value = hint.value;
    return;
  }
  const code = normalizeReferralCode(draft.value);
  if (!code) return;
  checking.value = true;
  try {
    const result = await CheckReferralCode(code, props.clientId);
    ownerName.value = result.ownerName;
    emit("update:modelValue", result.code);
    dialog.value = false;
  } catch (e) {
    serverError.value = errorText(e);
  } finally {
    checking.value = false;
  }
}

function remove() {
  emit("update:modelValue", "");
  ownerName.value = "";
  draft.value = "";
  dialog.value = false;
}

defineExpose({ loadInfo });
</script>

<style scoped>
.header-btn-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.header-btn-subtitle {
  font-size: 0.8rem;
  line-height: 1.2;
  color: rgba(var(--k-theme-on-surface), 0.6);
  margin-bottom: 2px;
}

.referral-btn {
  height: 35px;
  color: rgba(var(--k-theme-on-surface), 0.6);
}

.referral-btn.referral-has-code {
  color: rgb(245, 127, 23);
}

.referral-explain {
  margin-bottom: 12px;
  color: rgba(var(--k-theme-on-surface), 0.7);
}

.referral-message {
  margin-top: 8px;
  font-size: 0.9rem;
}
</style>
