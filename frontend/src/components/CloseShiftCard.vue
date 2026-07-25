<template>
  <v-card title="Shift Summary">
    <v-card-text>
      <template v-if="balanceRows.length">
        <div v-for="b in balanceRows" :key="b.accountId" class="d-flex justify-space-between py-1">
          <span>{{ b.accountName }}</span>
          <span class="font-weight-bold">${{ (b.balance / 100).toFixed(2) }}</span>
        </div>
      </template>
      <div v-else class="text-center text-medium-emphasis py-4">No transactions this shift</div>
    </v-card-text>
    <v-card-actions class="pa-2 ga-2">
      <v-btn variant="text" @click="emit('close')">Cancel</v-btn>
      <v-btn color="red-darken-4" variant="elevated" :loading="closing" @click="doCloseShift">
        Close Shift & Logout
      </v-btn>
    </v-card-actions>
  </v-card>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { GetOpenShift, ListShiftBalances, CloseShiftByID, OpenShift } from "../../wailsjs/go/main/App";

const emit = defineEmits(["close"]);
const auth = useAuthStore();
const router = useRouter();
const closing = ref(false);
const balanceRows = ref<any[]>([]);
let shiftId = 0;

onMounted(async () => {
  try {
    const shift = await GetOpenShift();
    shiftId = shift.id as number;
    const b = await ListShiftBalances(shiftId);
    balanceRows.value = (b as any[]) ?? [];
  } catch {}
});

async function doCloseShift() {
  closing.value = true;
  try {
    if (shiftId) await CloseShiftByID(shiftId, balanceRows.value);
    await OpenShift();
  } catch (e) {
    console.error("close shift:", e);
  }
  closing.value = false;
  auth.logout();
  router.push("/login");
}
</script>
