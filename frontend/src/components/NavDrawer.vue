<template>
  <v-navigation-drawer permanent width="95">
    <v-list class="d-flex flex-column fill-height pa-0" nav>
      <v-list-item
        v-for="item in items"
        :key="item.value"
        class="nav-item"
        :disabled="item.locked"
        :to="item.locked ? undefined : item.to"
        :value="item.value"
      >
        <div class="nav-item-content">
          <v-badge
            :model-value="!!item.locked"
            color="grey-darken-1"
            :icon="'mdi-lock'"
            location="bottom end"
            offset-x="-20"
            offset-y="4"
          >
            <v-icon :icon="item.icon" size="20" />
          </v-badge>
          <span class="text-caption text-center mt-1">{{ item.t }}</span>
        </div>
      </v-list-item>

      <v-list-item
        class="nav-item"
        :value="undefined"
        :active="false"
        @click="closeShiftDialog = true"
      >
        <div class="nav-item-content">
          <v-icon icon="mdi-close-circle-outline" size="20" />
          <span class="text-caption text-center mt-1">Close Shift</span>
        </div>
      </v-list-item>
    </v-list>
  </v-navigation-drawer>

  <v-dialog v-model="closeShiftDialog" max-width="600">
    <close-shift-card @close="closeShiftDialog = false" />
  </v-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from "vue";
import { useAuthStore } from "@/stores/auth";
import CloseShiftCard from "./CloseShiftCard.vue";

const auth = useAuthStore();
const closeShiftDialog = ref(false);

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
    {
      value: "attendance",
      to: "/attendance",
      icon: "mdi-account-multiple",
      t: "Attendance",
      locked: true,
    },
    { value: "settings", to: "/settings", icon: "mdi-cog", t: "Settings" },
  ];
  return all.filter((item) => item.value !== "settings" || auth.isAdmin);
});
</script>

<style scoped>
.nav-item {
  flex: 1 1 0;
  min-height: 0;
}

.nav-item.v-list-item--active {
  background: rgba(var(--v-theme-on-surface), 0.12);
  color: rgb(var(--v-theme-on-surface));
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
</style>
