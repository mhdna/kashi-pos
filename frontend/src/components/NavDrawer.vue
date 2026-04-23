<template>
  <v-navigation-drawer permanent width="172">
    <v-divider></v-divider>
    <v-list nav class="d-flex flex-column justify-space-evenly fill-height">
      <v-list-item v-for="i in items" :key="i.value" :value="i.value" :to="i.to">
        <div class="d-flex flex-column justify-center align-center" :class="i.class">
          <v-icon size="40" :class="i.iconClass">
            {{ i.icon }}
          </v-icon>
          <h3>{{ i.title }}</h3>
        </div>
      </v-list-item>
    </v-list>
  </v-navigation-drawer>
</template>

<script setup>
import { ref } from 'vue';

const items = ref([
  {
    value: "invoice",
    to: "/",
    icon: "mdi-invoice",
    title: "Invoice"
  },
  {
    value: "inventory",
    to: "/inventory",
    icon: "mdi-package-variant-closed",
    title: "Inventory"
  },
  {
    value: "transfer",
    to: "/transfers",
    icon: "mdi-truck",
    title: "Transfers",
    class: "bg-yellow",
    iconClass: "bounce"
  },
  {
    value: "clients",
    to: "/clients",
    icon: "mdi-account-multiple",
    title: "Clients"
  },
  {
    value: "attendance",
    to: "/attendance",
    icon: "mdi-badge-account",
    title: "Attendance"
  },
  {
    value: "shifts",
    to: "/shifts",
    icon: "mdi-timeline-clock",
    title: "Balance & Shifts"
  },
  {
    value: "close-shift",
    to: "/close-shift",
    icon: "mdi-close-circle-outline",
    title: "Close Shift"
  }
])
</script>

<style>
@keyframes bounce {

  0%,
  100% {
    transform: translateY(0);
  }

  50% {
    transform: translateY(8px);
  }
}

.bounce {
  animation: bounce 1.5s infinite;
}
</style>