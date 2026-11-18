<template>
  <div v-if="visible" class="header-btn-wrap">
    <div class="header-btn-subtitle">Rewards</div>
    <k-btn
      size="small"
      variant="text"
      aria-label="Rewards"
      class="rewards-btn"
      :class="{ 'rewards-on': activeCount > 0 }"
      data-test="rewards-open"
      @click="open"
    >
      <k-icon icon="mdi-gift-outline" size="20" class="flex-shrink-0" />
      <span v-if="activeCount > 0" class="rewards-badge" data-test="rewards-badge">{{ activeCount }}</span>
    </k-btn>
    <k-dialog v-model="dialog" max-width="620">
      <k-card>
        <k-card-title>Rewards</k-card-title>
        <k-card-text class="rewards-body">
          <section v-if="pickable.length" data-test="rewards-pickable">
            <div class="rewards-heading">For this sale</div>
            <div
              v-for="d in pickable"
              :key="d.uid"
              class="rewards-row"
              :data-test="`deal-${d.uid}`"
            >
              <div class="rewards-row-text">
                <div class="rewards-name">{{ d.name }}</div>
                <div class="rewards-terms">{{ dealTerms(d) }}</div>
              </div>
              <k-switch
                color="primary"
                hide-details
                inset
                :aria-label="d.name"
                :data-test="`pick-${d.uid}`"
                :model-value="picked.includes(d.uid)"
                @update:model-value="(on: boolean) => togglePick(d.uid, on)"
              />
            </div>
          </section>

          <section v-if="referralEnabled" data-test="rewards-referral">
            <div class="rewards-heading">Referral code</div>
            <p class="rewards-explain">
              A new customer can give the code of the client who sent them. That client gets
              {{ rewardPercent }}% off their next visit.
            </p>
            <div v-if="modelValue" class="rewards-row" data-test="referral-current">
              <div class="rewards-row-text">
                <div class="rewards-name">{{ modelValue }}</div>
                <div class="rewards-terms">{{ ownerName ? `Sent by ${ownerName}` : "Added to this sale" }}</div>
              </div>
              <k-btn variant="text" color="error" data-test="referral-remove" @click="removeCode">Remove</k-btn>
            </div>
            <template v-else>
              <div class="d-flex align-center ga-2">
                <k-text-field
                  v-model="draft"
                  class="flex-grow-1"
                  hide-details
                  placeholder="Code, like K7M4QH"
                  variant="outlined"
                  data-test="referral-input"
                  @keydown.enter="check"
                />
                <k-btn color="primary" variant="elevated" :loading="checking" data-test="referral-add" @click="check">Add</k-btn>
              </div>
              <div v-if="message" class="rewards-message" :class="messageError ? 'text-error' : ''" data-test="referral-message">
                {{ message }}
              </div>
              <InlineKeyboard v-model="draft" :max-length="12" class="mt-3" />
            </template>
          </section>

          <section v-if="automatic.length" data-test="rewards-automatic">
            <div class="rewards-heading">Apply by themselves</div>
            <div
              v-for="d in automatic"
              :key="d.uid"
              class="rewards-row"
              :data-test="`auto-${d.uid}`"
            >
              <div class="rewards-row-text">
                <div class="rewards-name">{{ d.name }}</div>
                <div class="rewards-terms">{{ dealTerms(d) }}</div>
              </div>
              <span v-if="d.name === appliedName" class="rewards-applied" data-test="auto-applied">On this sale</span>
            </div>
            <p class="rewards-note">Set in kashi; only kashi or an admin changes them.</p>
          </section>
        </k-card-text>
        <k-card-actions class="pa-4">
          <span v-if="appliedName" class="rewards-now" data-test="rewards-now">Now off this sale: {{ appliedName }}</span>
          <k-spacer />
          <k-btn color="primary" variant="elevated" data-test="rewards-done" @click="dialog = false">Done</k-btn>
        </k-card-actions>
      </k-card>
    </k-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from "vue";
import InlineKeyboard from "@/components/InlineKeyboard.vue";
import { CheckReferralCode, GetReferralInfo, ListRewardDeals } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { normalizeReferralCode, referralCodeHint } from "@/lib/referralCode";
import { dealTerms, rewardsActiveCount, type RewardDeal } from "@/lib/rewards";

// The sale's rewards: the deals the cashier picks (a student discount), the
// referral code a new customer gives, and, for reference, the deals that
// apply by themselves. modelValue is the referral code, picked the uids of
// the deals picked; appliedName is the invoice discount the quote applied.
const props = defineProps<{ modelValue: string; picked: string[]; clientId: number; appliedName?: string }>();
const emit = defineEmits<{ "update:modelValue": [code: string]; "update:picked": [uids: string[]] }>();

const referralEnabled = ref(false);
const rewardPercent = ref(0);
const deals = ref<RewardDeal[]>([]);
const dialog = ref(false);
const draft = ref("");
const checking = ref(false);
const serverError = ref("");
const ownerName = ref("");

const pickable = computed(() => deals.value.filter((d) => d.cashierPicks));
const automatic = computed(() => deals.value.filter((d) => !d.cashierPicks));
const visible = computed(() => referralEnabled.value || deals.value.length > 0);
const activeCount = computed(() =>
  rewardsActiveCount(
    props.modelValue,
    props.picked,
    props.appliedName ?? "",
    pickable.value.filter((d) => props.picked.includes(d.uid)).map((d) => d.name),
  ),
);

async function loadInfo() {
  try {
    const info = await GetReferralInfo(0);
    referralEnabled.value = Boolean(info.enabled);
    rewardPercent.value = Number(info.rewardPercent);
  } catch {
    referralEnabled.value = false;
  }
  try {
    deals.value = ((await ListRewardDeals()) ?? []) as RewardDeal[];
  } catch {
    deals.value = [];
  }
  // A deal central took away can't stay picked.
  const known = new Set(pickable.value.map((d) => d.uid));
  if (props.picked.some((uid) => !known.has(uid))) {
    emit("update:picked", props.picked.filter((uid) => known.has(uid)));
  }
}
void loadInfo();
// Central's changes reach the till as pulls; look again then, so the button
// and its deals follow without leaving the sale screen.
const stops = ["sync_applied:referral_program", "sync_applied:invoice_discounts"].map((name) =>
  EventsOn(name, () => void loadInfo()),
);
onUnmounted(() => stops.forEach((stop) => stop?.()));

function togglePick(uid: string, on: boolean) {
  const without = props.picked.filter((u) => u !== uid);
  emit("update:picked", on ? [...without, uid] : without);
}

const hint = computed(() => referralCodeHint(draft.value));
const message = computed(() => serverError.value || hint.value);
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
  draft.value = "";
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
    draft.value = "";
  } catch (e) {
    serverError.value = errorText(e);
  } finally {
    checking.value = false;
  }
}

function removeCode() {
  emit("update:modelValue", "");
  ownerName.value = "";
  draft.value = "";
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

.rewards-btn {
  position: relative;
  height: 35px;
  color: rgba(var(--k-theme-on-surface), 0.6);
}

.rewards-btn.rewards-on {
  color: rgb(245, 127, 23);
}

.rewards-badge {
  position: absolute;
  top: 0;
  right: 2px;
  min-width: 16px;
  font-size: 0.72rem;
  font-weight: 700;
  line-height: 16px;
  text-align: center;
  color: rgb(245, 127, 23);
}

.rewards-body {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.rewards-heading {
  font-weight: 600;
  margin-bottom: 8px;
}

.rewards-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
  border-top: 1px solid rgba(var(--k-theme-on-surface), 0.1);
}

.rewards-row-text {
  flex: 1 1 auto;
  min-width: 0;
}

.rewards-name {
  font-weight: 500;
}

.rewards-terms,
.rewards-note,
.rewards-explain {
  font-size: 0.9rem;
  color: rgba(var(--k-theme-on-surface), 0.65);
}

.rewards-explain {
  margin-bottom: 12px;
}

.rewards-note {
  margin-top: 6px;
}

.rewards-applied,
.rewards-now {
  font-weight: 600;
  color: rgb(46, 125, 50);
}

.rewards-message {
  margin-top: 8px;
  font-size: 0.9rem;
}
</style>
