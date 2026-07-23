<script setup lang="ts">
// import AbstractBG from "@/components/AbstractBG.vue";
import FloatingIcons from "@/components/FloatingIcons.vue";
import LoginForm from "@/components/LoginForm.vue";
</script>

<template>
  <!-- <AbstractBG /> -->

  <floating-icons />
  <div
    class="d-flex flex-column"
    style="
      position: relative;
      z-index: 1;
      height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
    "
  >
    <LoginForm />
  </div>
</template>
