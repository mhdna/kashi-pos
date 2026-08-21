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
        class="nav-item py-2"
        :value="undefined"
        :active="false"
        @click="logout"
      >
        <div class="nav-item-content">
          <v-icon icon="mdi-logout" size="20" />
          <span class="nav-item-label text-center mt-1">Logout</span>
        </div>
      </v-list-item>

      <v-list-item
        class="nav-item close-shift-item py-2"
        :value="undefined"
        :active="false"
        @click="closeShiftDialog = true"
      >
        <div class="nav-item-content">
          <v-icon icon="mdi-close-circle-outline" size="20" />
          <span class="nav-item-label text-center mt-1">Close Shift</span>
        </div>
      </v-list-item>
    </v-list>
  </v-navigation-drawer>

  <v-dialog v-model="closeShiftDialog" fullscreen>
    <close-shift-card @close="closeShiftDialog = false" />
  </v-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { pendingNavAction } from "@/composables/useNavActions";
import CloseShiftCard from "./CloseShiftCard.vue";

const auth = useAuthStore();
const route = useRoute();
const router = useRouter();
const closeShiftDialog = ref(false);

function handleItemClick(item: { locked?: boolean; action?: () => void; to?: string }) {
  if (item.locked) return;
  if (item.action) {
    item.action();
    return;
  }
  if (item.to) void router.push(item.to);
}

function onHoldClick() {
  pendingNavAction.value = "onhold";
}

function onExchangeClick() {
  pendingNavAction.value = "exchange";
}

function logout() {
  auth.logout();
  void router.push("/login");
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
    { value: "onhold", icon: "mdi-pause", t: "On Hold", action: onHoldClick },
    { value: "exchange", icon: "mdi-autorenew", t: "Exchange", action: onExchangeClick },
    {
      value: "clients",
      to: "/clients",
      icon: "mdi-account-multiple",
      t: "Clients",
      locked: !auth.isAdmin,
    },
    {
      value: "inventory",
      to: "/inventory",
      icon: "mdi-package-variant-closed",
      t: "Inventory",
      locked: true,
    },
    { value: "transfer", to: "/transfers", icon: "mdi-truck", t: "Transfers", locked: true },
    { value: "salespersons", to: "/salespersons", icon: "mdi-chart-bar", t: "Sales" },
    {
      value: "attendance",
      to: "/attendance",
      icon: "mdi-account-multiple",
      t: "Attend",
      locked: true,
    },
    { value: "settings", to: "/settings", icon: "mdi-cog", t: "Settings" },
  ];
  return all.filter((item) => item.value !== "settings" || auth.isAdmin);
});
</script>

<style scoped>
.nav-drawer {
  background-color: #0d47a1 !important;
  border-radius: 0 40px 40px 0 !important;
}

.nav-item {
  flex: 1 1 0;
  min-height: 0;
  color: #ffffff;
}

.nav-item:not(:last-child) {
  border-bottom: 1px solid rgba(255, 255, 255, 0.2);
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

.close-shift-item {
  border-radius: 0 0 40px 0;
}
</style>
