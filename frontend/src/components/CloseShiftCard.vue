<template>
  <div class="close-shift-content">
    <h1 class="text-h5 close-shift-title">Shift Summary{{ shiftId ? ` (#${shiftId})` : "" }}</h1>
    <div v-if="shiftCreatedAt" class="shift-times text-caption text-medium-emphasis">
      <span>Opened: {{ formatServerDateTime(shiftCreatedAt) }}</span>
      <span>Closing: {{ closingTime }}</span>
    </div>
    <div class="close-shift-body">
      <template v-if="balanceRows.length">
        <div class="balance-summary-grid">
          <div
            v-for="b in balanceRows"
            :key="b.accountId"
            class="d-flex justify-space-between py-1"
          >
            <span>
              {{ b.accountName }}
            </span>
            <span class="font-weight-bold">
              {{ formatBalance(b.balance, b.currencyCode ?? "USD", currencies, roundingMode) }}
            </span>
          </div>
        </div>
      </template>
      <div v-else class="text-center text-medium-emphasis py-4">No transactions this shift</div>
      <!-- Money paid out during the shift, listed under the sales balances
           (which stay as they are). -->
      <div v-if="shiftSpend.length" class="shift-spend mt-3">
        <div class="text-caption font-weight-bold text-medium-emphasis mb-1">Expenses &amp; loans this shift</div>
        <div v-for="row in shiftSpend" :key="row.key" class="d-flex justify-space-between py-1">
          <span class="text-truncate">
            <v-icon :icon="row.kind === 'loan' ? 'mdi-hand-coin-outline' : 'mdi-receipt-text-outline'" size="16" class="me-1" />
            {{ row.label }}
          </span>
          <span class="font-weight-bold text-error">−{{ formatSpendAmount(row.amount, row.currencyCode, currencies) }}</span>
        </div>
      </div>
      <v-select
        v-model="closingPersonId"
        :items="salespersons"
        item-title="fullName"
        item-value="id"
        label="Closing person"
        active
        :menu-props="{ transition: false }"
        variant="outlined"
        density="comfortable"
        class="mt-4"
      />
      <div class="autofill-actions d-flex justify-end align-end flex-column">
        <v-btn
          size="small"
          variant="tonal"
          prepend-icon="mdi-autorenew"
          :loading="autofilling"
          @click="autofillFromExpected"
        >
          Autofill (Estimated Amount)
        </v-btn>
      </div>
      <v-alert v-if="autofillError" type="error" variant="tonal" density="compact" class="mb-3">
        Autofill failed: {{ autofillError }}
      </v-alert>
      <div class="usd-bill-grid">
        <NumberPadMenu
          v-for="denomination in usdDenominations"
          :key="denomination"
          :model-value="usdBillCounts[denomination] || 0"
          :aria-label="`Count $${denomination} bills`"
          :transition="false"
          :min="0"
          :max="999"
          @update:model-value="
            (value) => {
              usdBillCounts[denomination] = String(value);
              autofilledBills[denomination] = false;
              editedBills[denomination] = true;
            }
          "
        >
          <template #activator="{ props: activatorProps }">
            <v-text-field
              v-bind="activatorProps"
              :model-value="usdBillCounts[denomination]"
              :label="`$${denomination}`"
              active
              suffix="bills"
              prepend-inner-icon="mdi-cash"
              readonly
              hide-details
              variant="outlined"
              density="compact"
              :class="{
                'autofilled-field': autofilledBills[denomination],
                'manually-edited-field': editedBills[denomination],
              }"
            />
          </template>
        </NumberPadMenu>
      </div>
      <div class="d-flex justify-space-between align-center mb-4 px-1">
        <span class="text-body-2">USD counted total</span>
        <span class="font-weight-bold">{{ formatUsd(countedUsdTotal / 100) }}</span>
      </div>
      <div class="counted-lbp-row">
        <NumberPadMenu
          :model-value="countedLbp"
          aria-label="Total LBP counted"
          :transition="false"
          :max="999999999999"
          suffix="LBP"
          @update:model-value="
            (value) => {
              countedLbp = String(value);
              autofilledLbp = false;
              editedLbp = true;
            }
          "
        >
          <template #activator="{ props: activatorProps }">
            <v-text-field
              v-bind="activatorProps"
              :model-value="countedLbp"
              label="LBP"
              active
              suffix="LBP"
              prepend-inner-icon="mdi-cash"
              readonly
              variant="outlined"
              density="comfortable"
              :class="{ 'autofilled-field': autofilledLbp, 'manually-edited-field': editedLbp }"
            />
          </template>
        </NumberPadMenu>
        <div class="d-flex justify-space-between align-center mb-2 px-1">
          <span class="text-body-2">LBP counted total</span>
          <span class="font-weight-bold">{{ countedLbpTotal }}</span>
        </div>
      </div>
      <div class="electronic-count-grid">
        <NumberPadMenu
          :model-value="countedVisa"
          aria-label="Counted Visa"
          :transition="false"
          allow-decimal
          :max="999999999.99"
          prefix="$"
          @update:model-value="
            (value) => {
              countedVisa = String(value);
              autofilledVisa = false;
              editedVisa = true;
            }
          "
        >
          <template #activator="{ props: activatorProps }">
            <v-text-field
              v-bind="activatorProps"
              :model-value="countedVisa"
              label="Visa"
              active
              prefix="$"
              prepend-inner-icon="mdi-credit-card-outline"
              readonly
              variant="outlined"
              density="comfortable"
              :class="{ 'autofilled-field': autofilledVisa, 'manually-edited-field': editedVisa }"
            />
          </template>
        </NumberPadMenu>
        <NumberPadMenu
          :model-value="countedWhish"
          aria-label="Counted Whish"
          :transition="false"
          allow-decimal
          :max="999999999.99"
          prefix="$"
          @update:model-value="
            (value) => {
              countedWhish = String(value);
              autofilledWhish = false;
              editedWhish = true;
            }
          "
        >
          <template #activator="{ props: activatorProps }">
            <v-text-field
              v-bind="activatorProps"
              :model-value="countedWhish"
              label="Whish"
              active
              prefix="$"
              prepend-inner-icon="mdi-cellphone"
              readonly
              variant="outlined"
              density="comfortable"
              :class="{ 'autofilled-field': autofilledWhish, 'manually-edited-field': editedWhish }"
            />
          </template>
        </NumberPadMenu>
        <v-text-field
          :model-value="(countedCouponCents / 100).toFixed(2)"
          label="Coupon"
          active
          prefix="$"
          prepend-inner-icon="mdi-ticket-percent-outline"
          persistent-hint
          readonly
          variant="outlined"
          density="comfortable"
        />
      </div>
      <div class="d-flex justify-space-between align-center mb-2 px-1">
        <span class="text-body-2">Non-cash payments counted</span>
        <span class="font-weight-bold">{{
          formatUsd((countedVisaCents + countedWhishCents + countedCouponCents) / 100)
        }}</span>
      </div>
    </div>
    <div class="close-shift-actions">
      <v-btn variant="text" @click="emit('close')">Cancel</v-btn>
      <v-btn
        color="error"
        variant="elevated"
        :loading="closing"
        :disabled="!formValid"
        @click="doCloseShift"
      >
        Close Shift
      </v-btn>
    </div>

    <CloseShiftSyncDialog ref="syncDialog" />

    <v-dialog v-model="receiptDialog" max-width="420" persistent scrollable>
      <v-card class="text-center">
        <template v-if="receiptState === 'scan'">
          <v-card-title class="pt-4">Upload the shift receipt</v-card-title>
          <v-card-text class="pb-4">
            <div class="text-body-2 mb-3">
              Scan with a phone and upload one photo of the printed close-shift receipt.
            </div>
            <img
              v-if="receiptQr"
              :src="receiptQr"
              alt="Upload QR code"
              style="width: 240px; height: 240px"
            />
            <div class="text-caption text-medium-emphasis mt-3" style="word-break: break-all">
              {{ receiptUrl }}
            </div>
            <div class="d-flex align-center justify-center ga-2 mt-4 text-medium-emphasis">
              <v-progress-circular indeterminate size="18" width="2" />
              <span class="text-caption">Waiting for the photo…</span>
            </div>

            <template v-if="!showSkip">
              <v-btn
                v-if="receiptMode === 'central'"
                block
                class="mt-4"
                :loading="switchingLocal"
                size="small"
                variant="text"
                @click="switchToLocalUpload"
              >
                Phone can't reach the server? Use local Wi-Fi
              </v-btn>
              <v-btn block class="mt-2" size="small" variant="text" @click="showSkip = true">
                Can't upload now
              </v-btn>
            </template>

            <div v-else class="d-flex flex-column ga-3 mt-4 text-left">
              <v-text-field
                v-model="adminPassword"
                autofocus
                density="compact"
                hide-details="auto"
                label="Admin password"
                type="password"
                variant="outlined"
                @update:model-value="skipError = ''"
              />
              <v-text-field
                v-model="skipReason"
                density="compact"
                hide-details="auto"
                label="Reason (optional)"
                variant="outlined"
              />
              <div v-if="skipError" class="text-caption text-error">{{ skipError }}</div>
              <div class="d-flex ga-2 justify-space-between">
                <v-btn size="small" variant="text" @click="showSkip = false">Back</v-btn>
                <v-btn
                  color="error"
                  :disabled="!adminPassword"
                  :loading="skipping"
                  size="small"
                  @click="submitSkip"
                >
                  Skip &amp; finish
                </v-btn>
              </div>
            </div>
          </v-card-text>
        </template>

        <template v-else>
          <v-card-text>
            <v-icon class="my-2" color="success" icon="mdi-check-circle" size="72" />
            <div class="text-h6">Receipt photo received</div>
            <div class="text-caption text-medium-emphasis mt-1">
              It will sync to the office automatically.
            </div>
          </v-card-text>
          <v-card-actions class="justify-center">
            <v-btn color="primary" variant="elevated" @click="finishReceipt">Finish</v-btn>
          </v-card-actions>
        </template>
      </v-card>
    </v-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { formatBalance, formatUsd } from "@/lib/format";
import { formatServerDateTime } from "@/lib/datetime";
import { playErrorSound, playSuccessSound } from "@/lib/sound";
import { useCurrencies } from "@/composables/useCurrencies";
import NumberPadMenu from "@/components/NumberPadMenu.vue";
import CloseShiftSyncDialog from "@/components/shifts/CloseShiftSyncDialog.vue";
import QRCode from "qrcode";
import {
  GetOpenShift,
  ListShiftBalances,
  ListSalespersons,
  CloseShiftByID,
  OpenShift,
  GetShiftExpectedTotals,
  CheckShiftReceiptCaptured,
  SkipShiftReceiptUpload,
  SwitchShiftReceiptToLocal,
  GetSpendSummary,
} from "../../wailsjs/go/main/App";
import { formatSpendAmount } from "@/lib/spend";
import { EventsOn } from "../../wailsjs/runtime/runtime";

const emit = defineEmits(["close"]);
const auth = useAuthStore();
const router = useRouter();
const closing = ref(false);
const balanceRows = ref<any[]>([]);
const salespersons = ref<any[]>([]);
const closingPersonId = ref<number | null>(null);
const countedLbp = ref("");
const countedVisa = ref("");
const countedWhish = ref("");
const usdDenominations = [100, 50, 20, 10, 5, 1] as const;
const usdBillCounts = ref<Record<number, string>>(
  Object.fromEntries(usdDenominations.map((denomination) => [denomination, ""])),
);
const { currencies, roundingMode } = useCurrencies();
const shiftId = ref(0);
const shiftCreatedAt = ref<string | null>(null);
const closingTime = ref(formatServerDateTime(new Date().toISOString()));
let closingClockTimer: ReturnType<typeof setInterval> | undefined;
const countedUsdTotal = computed(() =>
  usdDenominations.reduce(
    (total, denomination) =>
      total + denomination * Number(usdBillCounts.value[denomination] || 0) * 100,
    0,
  ),
);
const countedLbpTotal = computed(
  () => `${Math.round(Number(countedLbp.value || 0)).toLocaleString()} LBP`,
);
const countedVisaCents = computed(() => Math.round(Number(countedVisa.value || 0) * 100));
const countedWhishCents = computed(() => Math.round(Number(countedWhish.value || 0) * 100));
const countedCouponCents = computed(() =>
  balanceRows.value
    .filter(
      (balance) => balance.isOnCoupon || balance.accountName?.trim().toLowerCase() === "on coupon",
    )
    .reduce((total, balance) => total + Number(balance.balance || 0), 0),
);
const formValid = computed(() => {
  const lbp = Number(countedLbp.value);
  return (
    Boolean(closingPersonId.value) &&
    countedLbp.value !== "" &&
    Number.isFinite(lbp) &&
    lbp >= 0 &&
    Number.isFinite(countedVisaCents.value) &&
    countedVisaCents.value >= 0 &&
    Number.isFinite(countedWhishCents.value) &&
    countedWhishCents.value >= 0 &&
    usdDenominations.every((denomination) => {
      const raw = usdBillCounts.value[denomination] ?? "";
      if (raw === "") return true;
      const count = Number(raw);
      return Number.isInteger(count) && count >= 0;
    })
  );
});

const autofilling = ref(false);
const autofillError = ref("");
const autofilledLbp = ref(false);
const autofilledVisa = ref(false);
const autofilledWhish = ref(false);
const autofilledBills = ref<Record<number, boolean>>(
  Object.fromEntries(usdDenominations.map((denomination) => [denomination, false])),
);
const editedLbp = ref(false);
const editedVisa = ref(false);
const editedWhish = ref(false);
const editedBills = ref<Record<number, boolean>>(
  Object.fromEntries(usdDenominations.map((denomination) => [denomination, false])),
);

// Prefills counted fields with the system's live expected totals (from
// transactions recorded so far) as a starting point - every field stays
// editable via its NumberPadMenu, since this is only a suggestion, not a
// substitute for actually counting the drawer.
async function autofillFromExpected() {
  if (!shiftId.value) return;
  autofilling.value = true;
  autofillError.value = "";
  try {
    const totals = await GetShiftExpectedTotals(shiftId.value);
    // Expected can legitimately be negative (e.g. refunds exceeded sales in
    // that currency this shift) - counted cash can't go negative, so clamp
    // the suggestion at 0 rather than leaving the field blank, which would
    // look like autofill did nothing.
    const expectedLbp = Math.max(0, Math.round(Number(totals.expectedLbp) || 0));
    const expectedVisaCents = Math.max(0, Math.round(Number(totals.expectedVisa) || 0));
    const expectedWhishCents = Math.max(0, Math.round(Number(totals.expectedWhish) || 0));
    countedLbp.value = String(expectedLbp);
    autofilledLbp.value = true;
    editedLbp.value = false;
    countedVisa.value = (expectedVisaCents / 100).toFixed(2);
    autofilledVisa.value = true;
    editedVisa.value = false;
    countedWhish.value = (expectedWhishCents / 100).toFixed(2);
    autofilledWhish.value = true;
    editedWhish.value = false;
    const suggestedBillCounts = (totals.usdBillCounts ?? {}) as Record<string, number>;
    for (const denomination of usdDenominations) {
      const count = Number(suggestedBillCounts[String(denomination)] ?? 0);
      usdBillCounts.value[denomination] = count > 0 ? String(count) : "";
      autofilledBills.value[denomination] = count > 0;
      editedBills.value[denomination] = false;
    }
  } catch (error) {
    countedLbp.value = "";
    autofilledLbp.value = false;
    editedLbp.value = false;
    countedVisa.value = "";
    autofilledVisa.value = false;
    editedVisa.value = false;
    countedWhish.value = "";
    autofilledWhish.value = false;
    editedWhish.value = false;
    for (const denomination of usdDenominations) {
      usdBillCounts.value[denomination] = "";
      autofilledBills.value[denomination] = false;
      editedBills.value[denomination] = false;
    }
    autofillError.value = error instanceof Error ? error.message : String(error);
  } finally {
    autofilling.value = false;
  }
}

// The shift's expenses and loans, listed under its balances.
const shiftSpend = ref<{ key: string; kind: "expense" | "loan"; label: string; amount: number; currencyCode: string }[]>([]);
async function loadShiftSpend() {
  try {
    const summary = await GetSpendSummary(shiftId.value);
    shiftSpend.value = [
      ...((summary.shiftExpenses ?? []) as any[]).map((e) => ({
        key: `e-${e.id}`, kind: "expense" as const, label: e.description || e.category || "Expense",
        amount: e.amount, currencyCode: e.currencyCode,
      })),
      ...((summary.shiftLoans ?? []) as any[]).map((l) => ({
        key: `l-${l.id}`, kind: "loan" as const, label: l.description || l.category || "Loan",
        amount: l.amount, currencyCode: l.currencyCode,
      })),
    ];
  } catch {
    shiftSpend.value = [];
  }
}

onMounted(async () => {
  try {
    const shift = await GetOpenShift();
    shiftId.value = shift.id as number;
    shiftCreatedAt.value = shift.createdAt ?? null;
    const b = await ListShiftBalances(shiftId.value);
    balanceRows.value = (b as any[]) ?? [];
    void loadShiftSpend();
    salespersons.value = ((await ListSalespersons()) as any[]) ?? [];
    await autofillFromExpected();
  } catch {}
});

onMounted(() => {
  closingClockTimer = setInterval(() => {
    closingTime.value = formatServerDateTime(new Date().toISOString());
  }, 1000);
});

onBeforeUnmount(() => {
  if (closingClockTimer) clearInterval(closingClockTimer);
});

type ReceiptUpload = { token: string; url: string; required: boolean; mode?: string };

const syncDialog = ref<InstanceType<typeof CloseShiftSyncDialog> | null>(null);

// With "upload everything before closing" on, nothing is closed or printed
// until the office has every sale -- the sync dialog uploads and shows
// progress first. It resolves straight away when the mode is off.
async function syncBeforeClose(): Promise<boolean> {
  try {
    return (await syncDialog.value?.run(shiftId.value)) ?? true;
  } catch (e) {
    console.error("close shift sync:", e);
    playErrorSound();
    window.alert(`Couldn't check what's left to upload: ${e instanceof Error ? e.message : e}`);
    return false;
  }
}

async function doCloseShift() {
  closing.value = true;
  if (!(await syncBeforeClose())) {
    closing.value = false;
    return;
  }
  let receiptUpload: ReceiptUpload | null = null;
  let shiftClosed = false;
  try {
    if (shiftId.value && closingPersonId.value) {
      const result = await CloseShiftByID(
        shiftId.value,
        closingPersonId.value,
        Math.round(Number(countedLbp.value)),
        countedVisaCents.value,
        countedWhishCents.value,
        JSON.stringify(
          Object.fromEntries(
            usdDenominations.map((denomination) => [
              denomination.toString(),
              Number(usdBillCounts.value[denomination] || 0),
            ]),
          ),
        ),
      );
      shiftClosed = true;
      if (result?.receiptError) {
        window.alert(`Shift closed, but the receipt could not be printed: ${result.receiptError}`);
      }
      receiptUpload = (result?.receiptUpload ?? null) as ReceiptUpload | null;
    }
    await OpenShift();
    playSuccessSound();
  } catch (e) {
    console.error("close shift:", e);
    playErrorSound();
    if (!shiftClosed) {
      // Still open: stay here so the cashier can retry (e.g. a sale was
      // rung up on another screen after the upload finished).
      closing.value = false;
      window.alert(`The shift was not closed: ${e instanceof Error ? e.message : e}`);
      return;
    }
  }
  closing.value = false;

  // The shift is closed. If a printed receipt exists, block the return to
  // login until the cashier uploads one photo of it (or a manager overrides).
  // The photo is buffered on this machine and forwarded to the office in the
  // background, so this step works with no internet.
  if (receiptUpload?.required) {
    openReceiptDialog(receiptUpload);
    return;
  }
  auth.logout();
  router.push("/login");
}

// ----------------------- Close-shift receipt photo -----------------------

const receiptDialog = ref(false);
const receiptState = ref<"scan" | "done">("scan");
const receiptUrl = ref("");
const receiptQr = ref("");
const receiptMode = ref<"central" | "local">("local");
const switchingLocal = ref(false);
const showSkip = ref(false);
const skipping = ref(false);
const adminPassword = ref("");
const skipReason = ref("");
const skipError = ref("");
let receiptToken = "";
let receiptPoll: ReturnType<typeof setInterval> | null = null;
let receiptUnsub: (() => void) | null = null;

async function renderReceiptQr(url: string) {
  try {
    receiptQr.value = await QRCode.toDataURL(url, { width: 240, margin: 1 });
  } catch {
    receiptQr.value = "";
  }
}

function stopReceiptWatchers() {
  if (receiptPoll !== null) {
    clearInterval(receiptPoll);
    receiptPoll = null;
  }
  receiptUnsub?.();
  receiptUnsub = null;
}

function markReceiptCaptured() {
  receiptState.value = "done";
  stopReceiptWatchers();
}

async function openReceiptDialog(upload: { token: string; url: string; mode?: string }) {
  stopReceiptWatchers();
  receiptToken = upload.token;
  receiptUrl.value = upload.url;
  receiptMode.value = upload.mode === "central" ? "central" : "local";
  receiptState.value = "scan";
  showSkip.value = false;
  adminPassword.value = "";
  skipReason.value = "";
  skipError.value = "";
  receiptDialog.value = true;

  await renderReceiptQr(upload.url);

  receiptUnsub = EventsOn(`shift_receipt_captured:${receiptToken}`, markReceiptCaptured);
  receiptPoll = setInterval(async () => {
    try {
      if (await CheckShiftReceiptCaptured(receiptToken)) markReceiptCaptured();
    } catch {
      // Offline / transient -- keep polling; the event or a later tick gets there.
    }
  }, 4000);
}

function finishReceipt() {
  stopReceiptWatchers();
  receiptDialog.value = false;
  auth.logout();
  router.push("/login");
}

// Fallback for when the phone can't reach the central server (shop internet
// dropped after close): re-point the QR at this POS's own upload server.
async function switchToLocalUpload() {
  switchingLocal.value = true;
  try {
    const res = await SwitchShiftReceiptToLocal(receiptToken);
    receiptToken = (res?.token as string) ?? receiptToken;
    receiptUrl.value = (res?.url as string) ?? receiptUrl.value;
    receiptMode.value = "local";
    await renderReceiptQr(receiptUrl.value);
  } catch (e) {
    console.error("switch to local upload:", e);
  } finally {
    switchingLocal.value = false;
  }
}

async function submitSkip() {
  skipError.value = "";
  skipping.value = true;
  try {
    await SkipShiftReceiptUpload(receiptToken, adminPassword.value, skipReason.value);
    finishReceipt();
  } catch (e: any) {
    skipError.value = e?.message ?? String(e);
  } finally {
    skipping.value = false;
  }
}

onBeforeUnmount(stopReceiptWatchers);
</script>

<style scoped>
.close-shift-content {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.close-shift-title {
  padding: 4px 16px 4px;
}

.shift-times {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 16px;
  padding: 0 16px 4px;
}

.close-shift-body {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  padding: 0 16px 2px;
}

.close-shift-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  flex: 0 0 auto;
  padding: 8px 16px 8px;
}

.autofilled-field :deep(.v-field) {
  background-color: rgba(255, 235, 59, 0.3);
}

.manually-edited-field :deep(.v-field) {
  background-color: rgba(76, 175, 80, 0.25);
}

.autofill-actions {
  margin-bottom: 32px;
}

.usd-bill-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 12px;
}

.balance-summary-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 24px;
  margin-bottom: 12px;
}

.electronic-count-grid {
  display: grid;
  gap: 0 12px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

@media (max-width: 700px) {
  .usd-bill-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .balance-summary-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .electronic-count-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
