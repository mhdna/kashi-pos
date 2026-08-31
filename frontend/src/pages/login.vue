<script setup lang="ts">
import LoginForm from "@/components/LoginForm.vue";
</script>

<template>
  <div class="login-page">
    <div class="login-content">
      <LoginForm />
    </div>
  </div>
</template>

<style scoped>
.login-page {
  position: relative;
  height: 100dvh;
  overflow: hidden;
  background: transparent;
}

.login-content {
  position: relative;
  z-index: 1;
  height: 100%;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}
</style>
