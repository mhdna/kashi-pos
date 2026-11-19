<template>
  <div v-if="visible" class="rewards-pair">
  <div class="header-btn-wrap">
    <div class="header-btn-subtitle">Sales</div>
    <k-btn
      size="small"
      variant="text"
      aria-label="Sales"
      class="rewards-btn"
      :class="{ 'rewards-on': activeCount > 0 }"
      data-test="rewards-open"
      @click="open"
    >
      <k-icon icon="mdi-gift" size="22" class="flex-shrink-0" />
      <span v-if="activeCount > 0" class="rewards-badge" data-test="rewards-badge">{{ activeCount }}</span>
    </k-btn>
  </div>
  <!-- The referral code has its own button beside Rewards: it opens the code
       straight away, without going through the Rewards dialog. -->
  <div v-if="referralEnabled" class="header-btn-wrap">
    <div class="header-btn-subtitle">Referral</div>
    <k-btn
      size="small"
      variant="text"
      aria-label="Referral code"
      class="rewards-btn"
      :class="{ 'rewards-on': Boolean(modelValue) }"
      data-test="referral-open"
      @click="openCode"
    >
      <k-icon icon="mdi-account-plus" size="22" class="flex-shrink-0" />
      <k-icon v-if="modelValue" icon="mdi-check-circle" size="14" class="referral-tick-badge" data-test="referral-open-tick" />
    </k-btn>
  </div>
    <k-dialog v-model="dialog" max-width="780">
      <k-card>
        <k-card-title>Sales</k-card-title>
        <k-card-text class="rewards-body">
          <div v-if="pickable.length" class="rewards-heading">Choose</div>
          <div v-if="pickable.length" class="rewards-grid" data-test="rewards-pick-grid">
            <button
              v-for="d in pickable"
              :key="d.uid"
              type="button"
              role="switch"
              class="reward-card reward-card--pick"
              :class="{ 'reward-card--on': picked.includes(d.uid) }"
              :aria-checked="picked.includes(d.uid)"
              :aria-label="d.name"
              :title="dealTerms(d)"
              :data-test="`deal-${d.uid}`"
              @click="togglePick(d.uid, !picked.includes(d.uid))"
            >
              <span class="reward-top">
                <span class="reward-line">{{ dealTitle(d) }}</span>
                <k-icon
                  :icon="picked.includes(d.uid) ? 'mdi-check-circle' : 'mdi-circle-outline'"
                  size="22"
                  class="reward-state-icon"
                  :data-test="`pick-${d.uid}`"
                />
              </span>
              <span class="reward-size">{{ dealSize(d) }}</span>
            </button>


          </div>

          <div v-if="automatic.length" class="rewards-grid rewards-grid--locked" data-test="rewards-automatic-grid">
            <div
              v-for="d in automatic"
              :key="d.uid"
              class="reward-card reward-card--locked"
              :class="{ 'reward-card--applied': applied.includes(d.name) }"
              :title="dealTerms(d)"
              :data-test="`auto-${d.uid}`"
            >
              <span class="reward-top">
                <span class="reward-line">{{ dealWhen(d) }}</span>
                <k-icon
                  v-if="applied.includes(d.name)"
                  icon="mdi-check-circle"
                  size="22"
                  class="reward-applied-icon"
                  data-test="auto-applied"
                />
                <k-icon v-else icon="mdi-lock-outline" size="20" class="reward-state-icon" />
              </span>
              <span class="reward-size">{{ dealSize(d) }}</span>
            </div>
          </div>
        </k-card-text>
        <k-card-actions class="pa-4">
          <k-spacer />
          <k-btn color="primary" variant="elevated" data-test="rewards-done" @click="dialog = false">Done</k-btn>
        </k-card-actions>
      </k-card>
    </k-dialog>
    <k-dialog v-model="codeDialog" max-width="560">
      <k-card>
        <k-card-title>Referral code</k-card-title>
        <k-card-text>
          <div v-if="modelValue" class="reward-referral-current" data-test="referral-current">
            <div>
              <div class="reward-line">{{ modelValue }}</div>
              <div class="rewards-explain">{{ ownerName ? `Sent by ${ownerName}` : "Added to this sale" }}</div>
              <div v-if="friendPercent > 0" class="rewards-explain text-error" data-test="referral-friend-off">{{ friendPercent }}% off this sale</div>
            </div>
            <k-btn variant="text" color="error" data-test="referral-remove" @click="removeCode">Remove</k-btn>
          </div>
          <template v-else>
            <div class="d-flex align-center ga-2">
              <k-text-field
                v-model="draft"
                autofocus
                class="flex-grow-1"
                hide-details
                placeholder="Code"
                variant="outlined"
                data-test="referral-input"
                @keydown.enter="check"
              />
              <k-btn color="primary" variant="elevated" :loading="checking" :disabled="draft.length < referralCodeLength" data-test="referral-add" @click="check">Send reward</k-btn>
            </div>
            <div v-if="message" class="rewards-message" :class="messageError ? 'text-error' : ''" data-test="referral-message">
              {{ message }}
            </div>
            <InlineKeyboard v-model="draft" :max-length="referralCodeLength" layout="caps" class="mt-3" />
          </template>
        </k-card-text>
        <k-card-actions class="pa-4">
          <k-spacer />
          <k-btn variant="text" data-test="referral-close" @click="codeDialog = false">Close</k-btn>
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
import { normalizeReferralCode, referralCodeHint, referralCodeInput, referralCodeLength } from "@/lib/referralCode";
import { appliedDealNames, dealSize, dealTerms, dealTitle, dealWhen, rewardsActiveCount, type RewardDeal } from "@/lib/rewards";

// The sale's rewards: the deals the cashier picks (a student discount), the
// referral code a new customer gives, and, for reference, the deals that
// apply by themselves. modelValue is the referral code, picked the uids of
// the deals picked; appliedName is the invoice discount the quote applied.
// stack: the branch lets deals add up (deals_stack), so several can be picked.
const props = defineProps<{ modelValue: string; picked: string[]; clientId: number; appliedName?: string; stack?: boolean }>();
const emit = defineEmits<{ "update:modelValue": [code: string]; "update:picked": [uids: string[]] }>();

const referralEnabled = ref(false);
const rewardPercent = ref(0);
// The new customer's percent off this sale, 0 when the program doesn't give one.
const friendPercent = ref(0);
const deals = ref<RewardDeal[]>([]);
const dialog = ref(false);
// The referral code is typed in a dialog of its own, opened from its card.
const codeDialog = ref(false);
const draft = ref("");
const checking = ref(false);
const serverError = ref("");
const ownerName = ref("");

const pickable = computed(() => deals.value.filter((d) => d.cashierPicks));
const applied = computed(() => appliedDealNames(props.appliedName ?? ""));
// Smallest minimum first: $50, $100, $150.
const automatic = computed(() => deals.value.filter((d) => !d.cashierPicks).sort((a, b) => a.minTotal - b.minTotal));
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
    friendPercent.value = Number(info.friendPercent ?? 0);
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

// One deal at a time: unless the branch lets deals add up, the till takes
// only the biggest off the invoice, so picking a deal replaces the one picked
// before (it takes over). Adding up, the cashier picks as many as apply.
function togglePick(uid: string, on: boolean) {
  if (props.stack) {
    const without = props.picked.filter((u) => u !== uid);
    emit("update:picked", on ? [...without, uid] : without);
    return;
  }
  emit("update:picked", on ? [uid] : []);
}

const hint = computed(() => referralCodeHint(draft.value));
const message = computed(() => serverError.value || hint.value);
const messageError = computed(() => Boolean(serverError.value) || /typo/.test(hint.value));
watch(draft, (value) => {
  serverError.value = "";
  // Only what a code can hold: capitals and digits, up to its length.
  const kept = referralCodeInput(value);
  if (kept !== value) draft.value = kept;
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

function openCode() {
  draft.value = "";
  serverError.value = "";
  codeDialog.value = true;
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
    codeDialog.value = false;
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
  codeDialog.value = false;
}

defineExpose({ loadInfo });
</script>

<style scoped>
.rewards-pair {
  display: flex;
  align-items: flex-start;
  gap: 0;
}
.referral-tick-badge {
  position: absolute;
  top: 0;
  right: 0;
  color: #2e7d32;
}
.header-btn-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.header-btn-subtitle {
  font-size: 0.72rem;
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
  color: rgb(46, 125, 50);
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
  color: rgb(46, 125, 50);
}

.rewards-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.rewards-heading {
  font-size: 0.9rem;
  font-weight: 600;
  color: rgba(var(--k-theme-on-surface), 0.6);
}

/* The locked deals sit on their own row, set apart by space alone. */
.rewards-grid--locked {
  margin-top: 8px;
}

.rewards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 10px;
}

.reward-card {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 12px;
  border: 2px solid rgba(var(--k-theme-on-surface), 0.12);
  text-align: left;
  color: inherit;
  font: inherit;
  background: none;
}

.reward-card--pick {
  cursor: pointer;
}

.reward-card--pick:active {
  transform: scale(0.98);
}

.reward-card--on {
  border-color: rgb(46, 125, 50);
}

.reward-card--on .reward-state-icon {
  color: rgb(46, 125, 50);
}

/* Locked deals are for reference: no border, so they don't read as buttons. */
.reward-card--locked {
  border-color: transparent;
}

.reward-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

/* The name is what the cashier looks for; the size reads under it. */
.reward-size {
  font-size: 1.05rem;
  font-weight: 600;
  color: rgb(var(--k-theme-error));
}

.reward-state-icon {
  color: rgba(var(--k-theme-on-surface), 0.4);
}

.reward-applied-icon,
.rewards-now {
  color: rgb(46, 125, 50);
}

.rewards-now {
  font-weight: 600;
}

.reward-line {
  font-size: 1.35rem;
  font-weight: 700;
  line-height: 1.2;
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.rewards-note,
.rewards-explain {
  font-size: 0.9rem;
  color: rgba(var(--k-theme-on-surface), 0.6);
}

.rewards-explain {
  margin-bottom: 12px;
}

.reward-referral-current {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.rewards-message {
  margin-top: 8px;
  font-size: 0.9rem;
}
</style>
