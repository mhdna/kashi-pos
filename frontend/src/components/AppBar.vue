<template>
  <v-app-bar color="red-darken-4">
    <template #prepend>
      <div class="d-flex flex-column align-last justify-center mx-2">
        <v-app-bar-title class="text-title-medium"
          >GB POS Terminal
        </v-app-bar-title>
        <div class="text-title-small">Main</div>
      </div>
      <div class="mx-8 me-0 text-title-small">{{ time }}</div>
      <!-- <div class="mx-8 text-title-small text-green pa-1 px-3"> -->
      <!--   <v-icon icon="mdi-cloud-check-variant" size="18" class="me-1" /> -->
      <!--   Synced -->
      <!-- </div> -->
    </template>
    <template #append>
      <ToggleTheme />
      <!-- <new-transfer-alert /> -->
      <salespersons-dialog />
    </template>
  </v-app-bar>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import ToggleTheme from "./ToggleTheme.vue";
import NewTransferAlert from "./NewTransferAlert.vue";
import SalespersonsDialog from "./SalespersonsDialog.vue";

const time = ref("");

function updateTime() {
  const now = new Date();
  const pad = (n: number) => n.toString().padStart(2, "0");
  time.value = `${pad(now.getDate())}-${pad(now.getMonth() + 1)}-${now.getFullYear()} ${pad(now.getHours())}:${pad(now.getMinutes())}`;
}

let interval: ReturnType<typeof setInterval>;
onMounted(() => {
  updateTime();
  interval = setInterval(updateTime, 1000);
});
onUnmounted(() => clearInterval(interval));
</script>
