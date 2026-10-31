<script setup lang="ts">
defineProps<{ vertical?: boolean }>();
</script>
<template><hr class="k-divider" :class="{ 'k-divider--vertical': vertical }" /></template>
