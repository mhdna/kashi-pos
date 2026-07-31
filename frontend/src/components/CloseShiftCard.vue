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
        <v-text-field
          v-for="denomination in usdDenominations"
          :key="denomination"
          v-model="usdBillCounts[denomination]"
          :placeholder="`$${denomination} bills`"
          type="number"
          min="0"
          step="1"
          variant="outlined"
          density="compact"
        />
      </div>
      <div class="d-flex justify-space-between align-center mb-4 px-1">
        <span class="text-body-2">USD counted total</span>
        <span class="font-weight-bold">{{ formatUsd(countedUsdTotal / 100) }}</span>
      </div>
      <v-text-field
        v-model="countedLbp"
        placeholder="Total LBP counted"
        suffix="LBP"
        type="number"
        min="0"
        step="1000"
        variant="outlined"
        density="comfortable"
      />
      <div class="d-flex justify-space-between align-center mb-2 px-1">
        <span class="text-body-2">LBP counted total</span>
        <span class="font-weight-bold">{{ countedLbpTotal }}</span>
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
const formValid = computed(() => {
  const lbp = Number(countedLbp.value);
  return (
    Boolean(closingPersonId.value) &&
    countedLbp.value !== "" &&
    Number.isFinite(lbp) &&
    lbp >= 0 &&
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
  gap: 0 12px;
}

</style>
