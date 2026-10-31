<script setup lang="ts">
defineProps<{ noGutters?: boolean }>();
</script>
<template><div class="k-row" :class="{ 'k-row--no-gutters': noGutters }"><slot /></div></template>
