<template>
  <v-navigation-drawer permanent width="72" class="nav-drawer border-0">
    <v-list class="d-flex flex-column fill-height pa-0" nav>
      <v-list-item
        v-for="item in items"
        :key="item.value"
        :class="['nav-item', { 'py-2': item.value === 'invoice', 'nav-item-alert': item.alert }]"
        :value="item.action ? undefined : item.value"
        :active="item.to ? route.path === item.to : item.action ? false : undefined"
        @click.prevent="handleItemClick(item)"
      >
        <div class="nav-item-content">
          <v-icon :icon="item.icon" size="24" />
          <span class="nav-item-label text-center mt-1">{{ item.t }}</span>
        </div>
      </v-list-item>

      <div v-if="visitorCounterEnabled" class="visitor-counter" aria-label="People visits today">
        <v-btn
          class="visitor-counter-btn"
          icon
          variant="flat"
          rounded="lg"
          aria-label="Increase visitor count"
          @click="adjustVisitors(1)"
        >
          <v-icon icon="mdi-chevron-up" size="32" />
        </v-btn>
        <span class="visitor-count">{{ visitorCount }}</span>
        <v-btn
          class="visitor-counter-btn"
          icon
          variant="flat"
          rounded="lg"
          aria-label="Decrease visitor count"
          @click="adjustVisitors(-1)"
        >
          <v-icon icon="mdi-chevron-down" size="32" />
        </v-btn>
      </div>

      <div
        v-if="auth.isAdmin"
        class="sync-dot-wrap"
        :title="syncDot.title"
        role="button"
        aria-label="Sync status"
        @click="openSyncSettings"
      >
        <v-icon icon="mdi-circle" size="12" :color="syncDot.color" />
      </div>

      <v-list-item
        class="nav-item options-item py-2"
        :value="undefined"
        :active="false"
        @click="optionsOpen = true"
      >
        <div class="nav-item-content">
          <v-icon icon="mdi-logout" size="24" />
          <span class="nav-item-label text-center mt-1">Out</span>
        </div>
      </v-list-item>
    </v-list>
  </v-navigation-drawer>

  <v-dialog v-model="optionsOpen" max-width="520">
    <v-card class="options-dialog-card" rounded="xl">
      <div class="dialog-close-row px-4 pt-0">
        <v-btn
          icon="mdi-close"
          variant="text"
          aria-label="Close out menu"
          @click="optionsOpen = false"
        />
      </div>

      <v-card-text class="options-grid options-grid--three px-4 pt-0 pb-4">
        <v-card class="option-card" rounded="lg" variant="tonal" @click="openCloseShift">
          <v-icon icon="mdi-close-circle-outline" size="44" />
          <span>Close Shift</span>
        </v-card>
        <v-card class="option-card" rounded="lg" variant="tonal" @click="logout">
          <v-icon icon="mdi-logout" size="44" />
          <span>Logout</span>
        </v-card>
        <v-card class="option-card" rounded="lg" variant="tonal" @click="closeApp">
          <v-icon icon="mdi-close" size="44" />
          <span>Close App</span>
        </v-card>
      </v-card-text>
    </v-card>
  </v-dialog>

  <v-dialog v-model="closeAppConfirmOpen" max-width="360">
    <v-card rounded="xl">
      <v-card-title>Close app</v-card-title>
      <v-card-text>Are you sure you want to close the app?</v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="closeAppConfirmOpen = false">Cancel</v-btn>
        <v-btn color="error" variant="flat" @click="confirmCloseApp">Close App</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="inventoryOpen" max-width="640">
    <v-card rounded="xl">
      <div class="dialog-close-row px-4 pt-0">
        <v-btn
          icon="mdi-close"
          variant="text"
          aria-label="Close inventory menu"
          @click="inventoryOpen = false"
        />
      </div>

      <v-card-text class="options-grid px-4 pt-0 pb-4">
        <v-card
          v-if="auth.isAdmin || pageUnlocks.inventory"
          class="option-card"
          rounded="lg"
          variant="tonal"
          @click="openInventoryPage"
        >
          <v-icon icon="mdi-package-variant-closed" size="44" />
          <span>Inventory</span>
        </v-card>
        <v-card
          v-if="auth.isAdmin || pageUnlocks.products"
          class="option-card"
          rounded="lg"
          variant="tonal"
          @click="openProducts"
        >
          <v-icon icon="mdi-tag-multiple-outline" size="44" />
          <span>Products</span>
        </v-card>
        <v-card
          v-if="auth.isAdmin || pageUnlocks.salespersons"
          class="option-card"
          rounded="lg"
          variant="tonal"
          @click="openSalespersons"
        >
          <v-icon icon="mdi-chart-bar" size="44" />
          <span>Sales</span>
        </v-card>
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { usePageUnlocks } from "@/composables/usePageUnlocks";
import { useVisitorCounter } from "@/composables/useVisitorCounter";
import { useSyncStatus } from "@/composables/useSyncStatus";
import { useAttendanceStatus } from "@/composables/useAttendanceStatus";
import { Quit } from "../../wailsjs/runtime/runtime";

const auth = useAuthStore();
const route = useRoute();
const router = useRouter();
const { pageUnlocks } = usePageUnlocks();
const {
  enabled: visitorCounterEnabled,
  count: visitorCount,
  adjust: adjustVisitors,
} = useVisitorCounter();

const { status: syncStatus } = useSyncStatus();
const syncDot = computed(() => {
  const st = syncStatus.value;
  if (!st || !st.configured) return { color: 'grey', title: 'Sync not set up' };
  if (st.failed > 0) return { color: 'error', title: `${st.failed} record(s) rejected by kashi` };
  if (st.pending > 0 || st.confirmedElsewhere > 0)
    return { color: 'warning', title: `${st.pending + st.confirmedElsewhere} record(s) not yet confirmed here` };
  return { color: 'success', title: 'Fully synced with kashi' };
});
const { status: attendanceStatus } = useAttendanceStatus();

function openSyncSettings() {
  router.push({ path: '/settings', query: { panel: 'general-sync' } });
}
const optionsOpen = ref(false);
const inventoryOpen = ref(false);
const closeAppConfirmOpen = ref(false);

function handleItemClick(item: { locked?: boolean; action?: () => void; to?: string }) {
  if (item.locked) return;
  if (item.action) {
    item.action();
    return;
  }
  if (item.to) void router.push(item.to);
}

function logout() {
  optionsOpen.value = false;
  auth.logout();
  void router.push("/login");
}

function closeApp() {
  optionsOpen.value = false;
  closeAppConfirmOpen.value = true;
}

function confirmCloseApp() {
  closeAppConfirmOpen.value = false;
  Quit();
}

function openCloseShift() {
  optionsOpen.value = false;
  void router.push("/close-shift");
}

function openInventory() {
  inventoryOpen.value = true;
}

function openInventoryPage() {
  if (!auth.isAdmin && !pageUnlocks.value.inventory) return;
  inventoryOpen.value = false;
  void router.push("/inventory");
}

function openProducts() {
  if (!auth.isAdmin && !pageUnlocks.value.products) return;
  inventoryOpen.value = false;
  void router.push("/products");
}

function openSalespersons() {
  if (!auth.isAdmin && !pageUnlocks.value.salespersons) return;
  inventoryOpen.value = false;
  void router.push("/salespersons");
}

const items = computed(() => {
  const all = [
    { value: "invoice", to: "/home", icon: "mdi-cash-register", t: "Invoice" },
    {
      value: "invoices-list",
      to: "/invoices-list",
      icon: "mdi-format-list-bulleted-square",
      t: "Invoices",
    },
    { value: "shifts", to: "/shifts", icon: "mdi-cash-fast", t: "Balance" },
    {
      value: "clients",
      to: "/clients",
      icon: "mdi-account-multiple",
      t: "Clients",
      locked: !auth.isAdmin && !pageUnlocks.value.clients,
    },
    {
      value: "inventory",
      icon: "mdi-package-variant-closed",
      t: "Inventory",
      action: openInventory,
      locked:
        !auth.isAdmin &&
        !pageUnlocks.value.inventory &&
        !pageUnlocks.value.products &&
        !pageUnlocks.value.salespersons,
    },
    {
      value: "transfers",
      to: "/transfers",
      icon: "mdi-truck",
      t: "Transfers",
      locked: !auth.isAdmin && !pageUnlocks.value.transfers,
    },
    {
      value: "attendance",
      to: "/attendance",
      icon: "mdi-account-clock",
      t: "Attend",
      locked: !auth.isAdmin && !pageUnlocks.value.attendance,
      alert: !attendanceStatus.value.reachable,
    },
    { value: "settings", to: "/settings", icon: "mdi-cog", t: "Settings" },
  ];
  return all.filter((item) => (item.value !== "settings" || auth.isAdmin) && !item.locked);
});
</script>

<style scoped>
.sync-dot-wrap {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 6px 0;
  cursor: pointer;
}
.nav-drawer {
  background-color: #0b2e6b;
  border: 0 !important;
  box-shadow: none !important;
}

.nav-item {
  flex: 1 1 0;
  min-height: 0;
  color: #ffffff;
}

.nav-item-content {
  align-items: center;
  display: flex;
  flex-direction: column;
  height: 100%;
  justify-content: center;
  width: 100%;
}

.nav-item :deep(.v-icon),
.nav-item :deep(.v-badge) {
  flex-shrink: 0;
  transform: translateZ(0);
}

.nav-item-alert {
  background-color: #c62828;
}

.nav-item-label {
  font-size: 0.8rem;
  font-weight: 700;
  line-height: 1.2;
}

.options-item {
  flex: 0 0 auto;
  min-height: 64px;
}

.visitor-counter {
  flex: 0 0 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  margin: 6px 8px;
  padding: 8px 0;
  border-radius: 12px;
  background-color: #f57c00;
  color: #ffffff;
}

.visitor-counter-btn {
  width: 40px;
  height: 40px;
  background-color: #d95d00 !important;
  color: #ffffff !important;
}

.visitor-counter-btn :deep(.v-icon) {
  font-size: 32px;
}

.visitor-count {
  font-size: 1.25rem;
  font-weight: 800;
  line-height: 1;
  min-width: 1.5rem;
  text-align: center;
}

.options-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.dialog-close-row {
  display: flex;
  justify-content: flex-end;
}

.options-grid--three {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.option-card {
  align-items: center;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 12px;
  justify-content: center;
  min-height: 144px;
  padding: 18px 12px;
  text-align: center;
  font-size: var(--kashi-font-size-lg);
  font-weight: 600;
  transition: background-color 0.15s ease;
}

.option-card:hover {
  background-color: rgba(var(--v-theme-primary), 0.12);
}
</style>
