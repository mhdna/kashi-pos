<template>
  <v-app>
    <v-main>
      <router-view />
    </v-main>
  </v-app>
</template>

<script setup>
import { GetTheme } from "@/wailsjs/go/main/App";
import { useTheme } from "vuetify";

const theme = useTheme();

GetTheme().then((saved) => {
  if (saved) theme.global.name.value = saved;
});
</script>
