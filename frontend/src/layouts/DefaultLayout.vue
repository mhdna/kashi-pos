<template>
  <!-- <AppBar /> -->
  <NavDrawer />
  <v-main class="app-main">
    <router-view />
  </v-main>

  <OnHoldMenu ref="onHoldMenuRef" hide-activator @restore="restoreHold" />
  <ReturnInvoiceMenu
    ref="returnInvoiceMenuRef"
    hide-activator
    :rounding-mode="roundingMode"
    @exchange="startExchange"
  />
</template>

<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { GetAppSettings } from "../../wailsjs/go/main/App";
import AppBar from "@/components/AppBar.vue";
import NavDrawer from "@/components/NavDrawer.vue";
import OnHoldMenu from "@/components/OnHoldMenu.vue";
import ReturnInvoiceMenu from "@/components/ReturnInvoiceMenu.vue";
import {
  pendingExchangeSelection,
  pendingHoldInvoice,
  pendingNavAction,
} from "@/composables/useNavActions";
import type { ExchangeSelection } from "@/lib/exchange";

const router = useRouter();
const roundingMode = ref("half_up");
const onHoldMenuRef = ref<InstanceType<typeof OnHoldMenu> | null>(null);
const returnInvoiceMenuRef = ref<InstanceType<typeof ReturnInvoiceMenu> | null>(null);

watch(pendingNavAction, async (action) => {
  if (!action) return;
  await nextTick();
  if (action === "onhold") onHoldMenuRef.value?.open();
  else returnInvoiceMenuRef.value?.open();
  pendingNavAction.value = null;
});

function restoreHold(id: number, items: any[]) {
  pendingHoldInvoice.value = { id, items };
  void router.push("/home");
}

function startExchange(selection: ExchangeSelection) {
  pendingExchangeSelection.value = selection;
  void router.push("/home");
}

onMounted(async () => {
  try {
    const settings = await GetAppSettings();
    roundingMode.value = settings?.roundingMode ?? "half_up";
  } catch {
    // Keep the default rounding mode if settings are unavailable.
  }
});
</script>

<style>
html,
body {
  overflow: hidden;
}
.app-main {
  height: 100vh;
}
.app-main :deep(.v-main__wrap) {
  height: 100%;
  overflow-y: auto;
}
</style>
