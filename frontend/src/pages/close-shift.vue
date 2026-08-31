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
  height: 100vh;
  min-height: 0;
  overflow: hidden;
  padding: 8px 24px 12px;
}

</style>
