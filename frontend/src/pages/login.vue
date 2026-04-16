<script setup lang="ts">
import AbstractBG from '@/components/AbstractBG.vue';
import LoginForm from '@/components/LoginForm.vue';
</script>

<template>
    <AbstractBG />
    <div
        style="position:relative; z-index:1;  height:100vh; display: flex; align-items: center; justify-content: center;">
        <LoginForm />
    </div>
</template>