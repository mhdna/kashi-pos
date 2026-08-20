<template>
  <v-dialog
    :model-value="modelValue"
    max-width="900"
    @update:model-value="$emit('update:modelValue', $event)"
  >
    <v-card :title="`Close Shift #${openShift?.id}`">
      <v-card-text>
        <div class="d-flex justify-space-between align-center mb-4 pa-3 rounded bg-surface-variant">
          <span class="text-subtitle-1 font-weight-bold">Shift income total</span>
          <span class="text-h6 font-weight-bold">{{
            formatBalance(shiftIncomeTotal, "USD", currencies, roundingMode)
          }}</span>
        </div>
        <div v-if="closeBalances.length" class="balance-summary-grid mb-4">
          <div
            v-for="balance in closeBalances"
            :key="balance.accountId"
            class="d-flex justify-space-between py-1"
          >
            <span>
              {{ balance.isOnAccount ? "(On Account)" : balance.accountName }}
              <small v-if="balance.isOnAccount" class="text-medium-emphasis">
                — not collected</small
              >
              <small v-else-if="balance.isOnCoupon" class="text-medium-emphasis">
                — automatically counted</small
              >
            </span>
            <span class="font-weight-bold">{{
              formatBalance(balance.balance, balance.currencyCode ?? "USD", currencies, roundingMode)
            }}</span>
          </div>
        </div>
        <div v-else class="text-center text-medium-emphasis mb-4">No transactions this shift</div>

        <v-select
          v-model="closingPersonId"
          :items="salespersons"
          item-title="fullName"
          item-value="id"
          label="Closing person"
          active
          variant="outlined"
          density="comfortable"
          class="mt-3"
        />

        <div class="text-subtitle-2 mb-2">USD bill count</div>
        <div class="usd-bill-grid">
          <v-text-field
            v-for="denomination in usdDenominations"
            :key="denomination"
            :model-value="usdBillCounts[denomination]"
            :label="`$${denomination}`"
            active
            readonly
            variant="outlined"
            density="compact"
            :prepend-inner-icon="denomination === 100 ? 'mdi-cash-100' : 'mdi-cash'"
            @click="openNumpad(`usd:${denomination}`)"
          />
        </div>
        <div class="d-flex justify-space-between align-center mb-4 px-1">
          <span class="text-body-2">USD counted total</span>
          <span class="text-subtitle-1 font-weight-bold">{{
            formatUsd(countedUsdTotal / 100)
          }}</span>
        </div>
        <div class="counted-lbp-row">
          <v-text-field
            :model-value="countedLbp"
          label="LBP"
          active
            prepend-inner-icon="mdi-cash"
            suffix="LBP"
            readonly
            variant="outlined"
            density="comfortable"
            @click="openNumpad('lbp')"
          />
          <div class="d-flex justify-space-between align-center mb-2 px-1">
            <span class="text-body-2">LBP counted total</span>
            <span class="text-subtitle-1 font-weight-bold">{{ countedLbpTotal }}</span>
          </div>
        </div>
        <div class="electronic-count-grid">
          <v-text-field
            :model-value="countedVisa"
            label="Visa"
            active
            prefix="$"
            readonly
            variant="outlined"
            density="comfortable"
            @click="openNumpad('visa')"
          />
          <v-text-field
            :model-value="countedWhish"
            label="Whish"
            active
            prefix="$"
            readonly
            variant="outlined"
            density="comfortable"
            @click="openNumpad('whish')"
          />
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
          <span class="text-subtitle-1 font-weight-bold">{{
            formatUsd((countedVisaCents + countedWhishCents + countedCouponCents) / 100)
          }}</span>
        </div>
      </v-card-text>
      <v-card-actions class="pa-2 ga-2">
        <v-btn variant="text" @click="$emit('update:modelValue', false)">Cancel</v-btn>
        <v-btn
          color="primary"
          variant="elevated"
          :loading="closing"
          :disabled="!formValid"
          @click="confirmClose"
        >
          Close & Print
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="numpadOpen" width="320">
    <v-card>
      <v-card-title>{{ numpadTitle }}</v-card-title>
      <v-card-text>
        <v-text-field
          :model-value="numpadValue || '0'"
          label="Amount"
          active
          readonly
          variant="outlined"
          class="mb-3"
        />
        <div class="numpad-grid">
          <v-btn
            v-for="key in numpadKeys"
            :key="key"
            size="large"
            variant="tonal"
            @click="pressNumpad(key)"
          >
            {{ key }}
          </v-btn>
          <v-btn
            size="large"
            variant="tonal"
            prepend-icon="mdi-backspace-outline"
            @click="backspaceNumpad"
            >Delete</v-btn
          >
          <v-btn size="large" variant="tonal" @click="pressNumpad('0')">0</v-btn>
          <v-btn size="large" color="primary" variant="elevated" @click="numpadOpen = false"
            >Done</v-btn
          >
        </div>
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import type { Balance, Shift } from "@/types/shifts";
import { formatBalance, formatUsd } from "@/lib/format";
import { useCurrencies } from "@/composables/useCurrencies";

const { currencies, roundingMode } = useCurrencies();

const props = defineProps<{
  modelValue: boolean;
  openShift: Shift | null;
  closeBalances: Balance[];
  shiftIncomeTotal: number;
  salespersons: Array<{ id: number; fullName: string }>;
  closing: boolean;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: boolean];
  confirm: [
    closingPersonId: number,
    countedLbp: number,
    countedVisa: number,
    countedWhish: number,
    usdBreakdownJson: string,
  ];
}>();

const closingPersonId = ref<number | null>(null);
const countedLbp = ref("");
const countedVisa = ref("");
const countedWhish = ref("");
const usdDenominations = [100, 50, 20, 10, 5, 1] as const;
const usdBillCounts = ref<Record<number, string>>(
  Object.fromEntries(usdDenominations.map((denomination) => [denomination, ""])),
);
const numpadOpen = ref(false);
const numpadTarget = ref("");
const numpadKeys = ["1", "2", "3", "4", "5", "6", "7", "8", "9", "."];
const numpadTitle = computed(() => {
  if (numpadTarget.value === "lbp") return "Counted LBP";
  if (numpadTarget.value === "visa") return "Counted Visa";
  if (numpadTarget.value === "whish") return "Counted Whish";
  return `$${numpadTarget.value.split(":")[1] || ""} bill count`;
});
const numpadValue = computed(() => {
  if (numpadTarget.value === "lbp") return countedLbp.value;
  if (numpadTarget.value === "visa") return countedVisa.value;
  if (numpadTarget.value === "whish") return countedWhish.value;
  return usdBillCounts.value[Number(numpadTarget.value.split(":")[1])] ?? "";
});
const countedVisaCents = computed(() => Math.round(Number(countedVisa.value || 0) * 100));
const countedWhishCents = computed(() => Math.round(Number(countedWhish.value || 0) * 100));
const countedCouponCents = computed(() =>
  props.closeBalances
    .filter(
      (balance) => balance.isOnCoupon || balance.accountName.trim().toLowerCase() === "on coupon",
    )
    .reduce((total, balance) => total + Number(balance.balance || 0), 0),
);

const parsedLbp = computed(() => Number(countedLbp.value));
const billCountsValid = computed(() =>
  usdDenominations.every((denomination) => {
    const raw = usdBillCounts.value[denomination] ?? "";
    if (raw === "") return true;
    const count = Number(raw);
    return Number.isInteger(count) && count >= 0;
  }),
);
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
const formValid = computed(
  () =>
    Boolean(closingPersonId.value) &&
    countedLbp.value !== "" &&
    Number.isFinite(parsedLbp.value) &&
    parsedLbp.value >= 0 &&
    Number.isFinite(countedVisaCents.value) &&
    countedVisaCents.value >= 0 &&
    Number.isFinite(countedWhishCents.value) &&
    countedWhishCents.value >= 0 &&
    billCountsValid.value,
);

watch(
  () => props.modelValue,
  (open) => {
    if (!open) return;
    closingPersonId.value = null;
    countedLbp.value = "";
    countedVisa.value = "";
    countedWhish.value = "";
    for (const denomination of usdDenominations) usdBillCounts.value[denomination] = "";
  },
);

function confirmClose() {
  if (!formValid.value || !closingPersonId.value) return;
  emit(
    "confirm",
    closingPersonId.value,
    Math.round(parsedLbp.value),
    countedVisaCents.value,
    countedWhishCents.value,
    JSON.stringify(
      Object.fromEntries([
        ...usdDenominations.map((denomination) => [
          denomination.toString(),
          Number(usdBillCounts.value[denomination] || 0),
        ]),
      ]),
    ),
  );
}

function openNumpad(target: string) {
  numpadTarget.value = target;
  numpadOpen.value = true;
}

function setNumpadValue(value: string) {
  if (numpadTarget.value === "lbp") countedLbp.value = value;
  else if (numpadTarget.value === "visa") countedVisa.value = value;
  else if (numpadTarget.value === "whish") countedWhish.value = value;
  else usdBillCounts.value[Number(numpadTarget.value.split(":")[1])] = value;
}

function pressNumpad(key: string) {
  const current = numpadValue.value;
  if (key === "." && current.includes(".")) return;
  if (key === "." && numpadTarget.value.startsWith("usd:")) return;
  setNumpadValue(current === "0" ? key : current + key);
}

function backspaceNumpad() {
  setNumpadValue(numpadValue.value.slice(0, -1));
}
</script>

<style scoped>
.usd-bill-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 0 12px;
}

.balance-summary-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 24px;
}

.electronic-count-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0 12px;
}

.numpad-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
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
