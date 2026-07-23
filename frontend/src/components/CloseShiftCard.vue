<template>
  <v-card class="d-flex flex-column" title="Shift Summary">
    <v-card-text class="flex-grow-1" />

    <v-card-actions class="pa-2 ga-2">
      <v-btn class="flex-1-1" variant="elevated"> Cancel </v-btn>
      <v-btn
        class="flex-1-1"
        color="red-darken-4"
        variant="elevated"
        @click="logout"
      >
        Close Shift
      </v-btn>
    </v-card-actions>
  </v-card>
</template>

<script setup>
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";

const auth = useAuthStore();
const router = useRouter();

function logout() {
  auth.logout();
  router.push("/login");
}

// import { Quit } from "../../wailsjs/runtime/runtime";
//
// function quit() {
//   Quit();
// }
</script>
