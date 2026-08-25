<template>
  <v-navigation-drawer
    permanent
    width="72"
    class="nav-drawer rounded-e-xl border-0"
  >
    <v-list class="d-flex flex-column fill-height pa-0" nav>
      <v-list-item
        v-for="item in items"
        :key="item.value"
        :class="['nav-item', { 'py-2': item.value === 'invoice' }]"
        :disabled="item.locked"
        :value="item.action ? undefined : item.value"
        :active="item.to ? route.path === item.to : item.action ? false : undefined"
        @click.prevent="handleItemClick(item)"
      >
        <div class="nav-item-content">
          <v-icon :icon="item.icon" size="20" />
          <span class="nav-item-label text-center mt-1">{{ item.t }}</span>
        </div>
      </v-list-item>

      <v-list-item
        class="nav-item options-item py-2"
        :value="undefined"
        :active="false"
        @click="optionsOpen = true"
      >
        <div class="nav-item-content">
          <v-icon icon="mdi-logout" size="20" />
          <span class="nav-item-label text-center mt-1">Out</span>
        </div>
      </v-list-item>
    </v-list>
  </v-navigation-drawer>

  <v-dialog v-model="optionsOpen" max-width="520">
    <v-card class="options-dialog-card" rounded="xl">
      <v-card-title class="d-flex align-center justify-space-between px-6 pt-5">
        <span>Out</span>
        <v-btn icon="mdi-close" variant="text" aria-label="Close out menu" @click="optionsOpen = false" />
      </v-card-title>

      <v-card-text class="options-grid options-grid--three px-6 pb-6">
        <v-card class="option-card" rounded="lg" variant="tonal" @click="logout">
          <v-icon icon="mdi-logout" size="32" />
          <span>Logout</span>
        </v-card>
        <v-card class="option-card" rounded="lg" variant="tonal" @click="openCloseShift">
          <v-icon icon="mdi-close-circle-outline" size="32" />
          <span>Close Shift</span>
        </v-card>
        <v-card class="option-card" rounded="lg" variant="tonal" @click="closeApp">
          <v-icon icon="mdi-close" size="32" />
          <span>Close App</span>
        </v-card>
      </v-card-text>
    </v-card>
  </v-dialog>

  <v-dialog v-model="moneyOpen" max-width="520">
    <v-card rounded="xl">
      <v-card-title class="d-flex align-center justify-space-between px-6 pt-5">
        <span>Money</span>
        <v-btn icon="mdi-close" variant="text" aria-label="Close money menu" @click="moneyOpen = false" />
      </v-card-title>

      <v-card-text class="options-grid px-6 pb-6">
        <v-card class="option-card" rounded="lg" variant="tonal" @click="openExpenses">
          <v-icon icon="mdi-receipt-text-outline" size="32" />
          <span>Expenses</span>
        </v-card>
        <v-card class="option-card" rounded="lg" variant="tonal" @click="openLoans">
          <v-icon icon="mdi-hand-coin-outline" size="32" />
          <span>Loans</span>
        </v-card>
      </v-card-text>
    </v-card>
  </v-dialog>

  <v-dialog v-model="inventoryOpen" max-width="640">
    <v-card rounded="xl">
      <v-card-title class="d-flex align-center justify-space-between px-6 pt-5">
        <span>Inventory</span>
        <v-btn icon="mdi-close" variant="text" aria-label="Close inventory menu" @click="inventoryOpen = false" />
      </v-card-title>

      <v-card-text class="options-grid options-grid--three px-6 pb-6">
        <v-card
          class="option-card"
          :disabled="!auth.isAdmin && !pageUnlocks.inventory"
          rounded="lg"
          variant="tonal"
          @click="openInventoryPage"
        >
          <v-icon icon="mdi-package-variant-closed" size="32" />
          <span>Inventory</span>
        </v-card>
        <v-card
          class="option-card"
          :disabled="!auth.isAdmin && !pageUnlocks.transfers"
          rounded="lg"
          variant="tonal"
          @click="openTransfers"
        >
          <v-icon icon="mdi-truck" size="32" />
          <span>Transfers</span>
        </v-card>
        <v-card
          class="option-card"
          :disabled="!auth.isAdmin && !pageUnlocks.salespersons"
          rounded="lg"
          variant="tonal"
          @click="openSalespersons"
        >
          <v-icon icon="mdi-chart-bar" size="32" />
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
import { Quit } from "../../wailsjs/runtime/runtime";

const auth = useAuthStore();
const route = useRoute();
const router = useRouter();
const { pageUnlocks } = usePageUnlocks();
const optionsOpen = ref(false);
const moneyOpen = ref(false);
const inventoryOpen = ref(false);

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
  if (window.confirm("Are you sure you want to close the app?")) {
    Quit();
  }
}

function openCloseShift() {
  optionsOpen.value = false;
  void router.push("/close-shift");
}

function openMoney() {
  moneyOpen.value = true;
}

function openExpenses() {
  moneyOpen.value = false;
  void router.push("/expenses");
}

function openLoans() {
  moneyOpen.value = false;
  void router.push("/loans");
}

function openInventory() {
  inventoryOpen.value = true;
}

function openInventoryPage() {
  if (!auth.isAdmin && !pageUnlocks.value.inventory) return;
  inventoryOpen.value = false;
  void router.push("/inventory");
}

function openTransfers() {
  if (!auth.isAdmin && !pageUnlocks.value.transfers) return;
  inventoryOpen.value = false;
  void router.push("/transfers");
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
    { value: "money", icon: "mdi-cash-multiple", t: "Money", action: openMoney },
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
        !pageUnlocks.value.transfers &&
        !pageUnlocks.value.salespersons,
    },
    {
      value: "attendance",
      to: "/attendance",
      icon: "mdi-account-multiple",
      t: "Attend",
      locked: !auth.isAdmin && !pageUnlocks.value.attendance,
    },
    { value: "settings", to: "/settings", icon: "mdi-cog", t: "Settings" },
  ];
  return all.filter((item) => item.value !== "settings" || auth.isAdmin);
});
</script>

<style scoped>
.nav-drawer {
  background-color: rgb(var(--v-theme-primary)) !important;
  border-radius: 0 40px 40px 0 !important;
}

.nav-item {
  flex: 1 1 0;
  min-height: 0;
  color: #ffffff;
}

.nav-item:first-child {
  border-radius: 0 40px 0 0;
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

.nav-item-label {
  font-size: var(--kashi-font-size-xs);
  line-height: 1.2;
}

.options-item {
  flex: 0 0 auto;
  min-height: 64px;
  border-radius: 0 0 40px 0;
}

.options-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.options-grid--three {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.option-card {
  align-items: center;
  aspect-ratio: 1;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 10px;
  justify-content: center;
  transition: transform 0.15s ease;
}

.option-card:hover {
  transform: translateY(-2px);
}
</style>
