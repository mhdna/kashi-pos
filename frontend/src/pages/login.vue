<script setup lang="ts">
import AbstractBG from "@/components/AbstractBG.vue";
import FloatingIcons from "@/components/FloatingIcons.vue";
import LoginForm from "@/components/LoginForm.vue";
import bg from "@/assets/bg.png";
</script>

<template>
  <div class="login-page">
    <!-- <AbstractBG /> -->
    <!-- <v-img :src="bg" cover class="login-background" /> -->
    <!-- <v-img :src="bg" cover class="login-background" /> -->

    <!-- <FloatingIcons /> -->
    <div class="login-content">
      <LoginForm />
    </div>
  </div>
</template>

<style scoped>
.login-page {
  position: relative;
  height: 100vh;
  overflow: hidden;
}

.login-background {
  position: absolute;
  inset: 0;
  z-index: 0;
}

.login-content {
  position: relative;
  z-index: 1;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
