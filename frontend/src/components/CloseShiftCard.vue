<template>
  <v-card title="Shift Summary">
    <v-card-text>
      <template v-if="balanceRows.length">
        <div v-for="b in balanceRows" :key="b.accountId" class="d-flex justify-space-between py-1">
          <span>{{ b.accountName }}</span>
          <span class="font-weight-bold">
            {{ formatBalance(b.balance, b.currencyCode ?? "USD", exchangeRate) }}
          </span>
        </div>
      </template>
      <div v-else class="text-center text-medium-emphasis py-4">No transactions this shift</div>
      <v-select
        v-model="closingPersonId"
        :items="salespersons"
        item-title="fullName"
        item-value="id"
        placeholder="Closing person"
        variant="outlined"
        density="comfortable"
        class="mt-4"
      />
      <div class="text-subtitle-2 mb-2">USD bill count</div>
      <div class="usd-bill-grid">
        <NumberPadMenu
          v-for="denomination in usdDenominations"
          :key="denomination"
          :model-value="usdBillCounts[denomination] || 0"
          :aria-label="`Count $${denomination} bills`"
          :min="0"
          :max="999"
          @update:model-value="(value) => (usdBillCounts[denomination] = String(value))"
        >
          <template #activator="{ props: activatorProps }">
            <v-text-field
              v-bind="activatorProps"
              :model-value="usdBillCounts[denomination]"
              :label="`$${denomination} bills`"
              placeholder="0"
              suffix="bills"
              prepend-inner-icon="mdi-cash"
              readonly
              hide-details
              variant="outlined"
              density="compact"
            />
          </template>
        </NumberPadMenu>
      </div>
      <div class="d-flex justify-space-between align-center mb-4 px-1">
        <span class="text-body-2">USD counted total</span>
        <span class="font-weight-bold">{{ formatUsd(countedUsdTotal / 100) }}</span>
      </div>
      <NumberPadMenu
        v-model="countedLbp"
        aria-label="Total LBP counted"
        :max="999999999999"
        suffix="LBP"
      >
        <template #activator="{ props: activatorProps }">
          <v-text-field
            v-bind="activatorProps"
            :model-value="countedLbp"
            label="Counted LBP"
            placeholder="0"
            suffix="LBP"
            prepend-inner-icon="mdi-cash"
            readonly
            variant="outlined"
            density="comfortable"
          />
        </template>
      </NumberPadMenu>
      <div class="d-flex justify-space-between align-center mb-2 px-1">
        <span class="text-body-2">LBP counted total</span>
        <span class="font-weight-bold">{{ countedLbpTotal }}</span>
      </div>
      <div class="electronic-count-grid">
        <NumberPadMenu
          v-model="countedVisa"
          aria-label="Counted Visa"
          allow-decimal
          :max="999999999.99"
          prefix="$"
        >
          <template #activator="{ props: activatorProps }">
            <v-text-field
              v-bind="activatorProps"
              :model-value="countedVisa"
              label="Counted Visa"
              placeholder="0.00"
              prefix="$"
              prepend-inner-icon="mdi-credit-card-outline"
              readonly
              variant="outlined"
              density="comfortable"
            />
          </template>
        </NumberPadMenu>
        <NumberPadMenu
          v-model="countedWhish"
          aria-label="Counted Whish"
          allow-decimal
          :max="999999999.99"
          prefix="$"
        >
          <template #activator="{ props: activatorProps }">
            <v-text-field
              v-bind="activatorProps"
              :model-value="countedWhish"
              label="Counted Whish"
              placeholder="0.00"
              prefix="$"
              prepend-inner-icon="mdi-cellphone"
              readonly
              variant="outlined"
              density="comfortable"
            />
          </template>
        </NumberPadMenu>
      </div>
      <div class="d-flex justify-space-between align-center mb-2 px-1">
        <span class="text-body-2">Electronic payments counted</span>
        <span class="font-weight-bold">{{ formatUsd((countedVisaCents + countedWhishCents) / 100) }}</span>
      </div>
    </v-card-text>
    <v-card-actions class="pa-2 ga-2">
      <v-btn variant="text" @click="emit('close')">Cancel</v-btn>
      <v-btn
        color="red-darken-4"
        variant="elevated"
        :loading="closing"
        :disabled="!formValid"
        @click="doCloseShift"
      >
        Close, Print & Logout
      </v-btn>
    </v-card-actions>
  </v-card>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { formatBalance, formatUsd } from "@/lib/format";
import NumberPadMenu from "@/components/NumberPadMenu.vue";
import {
  GetAppSettings,
  GetOpenShift,
  ListShiftBalances,
  ListSalespersons,
  CloseShiftByID,
  OpenShift,
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
const exchangeRate = ref(90000);
let shiftId = 0;
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

onMounted(async () => {
  try {
    const shift = await GetOpenShift();
    shiftId = shift.id as number;
    const b = await ListShiftBalances(shiftId);
    balanceRows.value = (b as any[]) ?? [];
    salespersons.value = ((await ListSalespersons()) as any[]) ?? [];
    const settings = await GetAppSettings();
    const configuredRate = Number(settings?.exchangeRate);
    if (Number.isFinite(configuredRate) && configuredRate > 0) {
      exchangeRate.value = configuredRate;
    }
  } catch {}
});

async function doCloseShift() {
  closing.value = true;
  try {
    if (shiftId && closingPersonId.value) {
      const result = await CloseShiftByID(
        shiftId,
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
.usd-bill-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 12px;
}

.electronic-count-grid {
  display: grid;
  gap: 0 12px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

</style>
