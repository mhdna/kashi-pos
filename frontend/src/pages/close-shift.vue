<template>
  <main class="close-shift-page">
    <CloseShiftCard @close="goBack" />
  </main>
</template>

<script setup lang="ts">
import { useRouter } from "vue-router";
import CloseShiftCard from "@/components/CloseShiftCard.vue";

const router = useRouter();

function goBack() {
  router.back();
}
</script>

<style scoped>
.close-shift-page {
  box-sizing: border-box;
  min-height: 100vh;
  overflow-y: auto;
  padding: 24px;
}

</style>
