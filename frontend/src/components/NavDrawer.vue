<template>
  <v-navigation-drawer permanent width="120">
    <v-list class="d-flex flex-column justify-space-evenly fill-height" nav>
      <v-list-item
        v-for="item in items"
        :key="item.value"
        class="flex-fill"
        :to="item.to"
        :value="item.value"
      >
        <div class="d-flex flex-column justify-center align-center pt-4">
          <v-icon :icon="item.icon" size="20" />
          <h5 class="text-center mt-1">{{ item.t }}</h5>
        </div>
      </v-list-item>
      <v-list-item
        :active="false"
        value="close_shift"
        @click="closeShiftDialog = true"
      >
        <div class="d-flex flex-column justify-center align-center">
          <v-icon icon="mdi-close-circle-outline" size="20" />
          <h5 class="text-center mt-1">Logout</h5>
        </div>
      </v-list-item>
    </v-list>
  </v-navigation-drawer>
  <v-dialog v-model="closeShiftDialog" max-width="600">
    <close-shift-card />
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
    { value: "settings", to: "/settings", icon: "mdi-cog", t: "Settings" },
    {
      value: "invoices-list",
      to: "/invoices-list",
      icon: "mdi-format-list-bulleted-square",
      t: "Invoice List",
    },
    {
      value: "inventory",
      to: "/inventory",
      icon: "mdi-package-variant-closed",
      t: "Inventory",
    },
    { value: "transfer", to: "/transfers", icon: "mdi-truck", t: "Transfers" },
    {
      value: "clients",
      to: "/clients",
      icon: "mdi-account-multiple",
      t: "Clients",
    },
    {
      value: "attendance",
      to: "/attendance",
      icon: "mdi-account-multiple",
      t: "Attendance",
    },
    {
      value: "shifts",
      to: "/shifts",
      icon: "mdi-cash-fast",
      t: "Balance & Shifts",
    },
  ];
  // show certain navigation items only for admin
  return all.filter((item) => item.value !== "settings" || auth.isAdmin);
});
</script>
