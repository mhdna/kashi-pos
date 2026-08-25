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
        <div class="text-caption text-medium-emphasis autofill-hint text-right">
          This is an estimate based on the shift's transactions.
        </div>
        <v-btn
          size="small"
          variant="tonal"
          prepend-icon="mdi-autorenew"
          :loading="autofilling"
          @click="autofillFromExpected"
        >
          Autofill from expected
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
        Close, Print & Logout
      </v-btn>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { formatBalance, formatUsd } from "@/lib/format";
import { formatServerDateTime } from "@/lib/datetime";
import { useCurrencies } from "@/composables/useCurrencies";
import NumberPadMenu from "@/components/NumberPadMenu.vue";
import {
  GetOpenShift,
  ListShiftBalances,
  ListSalespersons,
  CloseShiftByID,
  OpenShift,
  GetShiftExpectedTotals,
} from "../../wailsjs/go/main/App";

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

onMounted(async () => {
  try {
    const shift = await GetOpenShift();
    shiftId.value = shift.id as number;
    shiftCreatedAt.value = shift.createdAt ?? null;
    const b = await ListShiftBalances(shiftId.value);
    balanceRows.value = (b as any[]) ?? [];
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

async function doCloseShift() {
  closing.value = true;
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
      if (result?.receiptError) {
        window.alert(`Shift closed, but the receipt could not be printed: ${result.receiptError}`);
      }
    }
    await OpenShift();
  } catch (e) {
    console.error("close shift:", e);
  }
  closing.value = false;
  auth.logout();
  router.push("/login");
}
</script>

<style scoped>
.close-shift-content {
  width: 100%;
}

.close-shift-title {
  padding: 16px 16px 8px;
}

.shift-times {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 16px;
  padding: 0 16px 8px;
}

.close-shift-body {
  padding: 0 16px 2px;
}

.close-shift-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 8px 16px 16px;
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
