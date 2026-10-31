<script setup lang="ts">
import { toUnit } from "./util";
defineProps<{ width?: string | number; permanent?: boolean; touchless?: boolean }>();
</script>
<template>
  <nav class="k-navigation-drawer" :style="{ '--k-drawer-w': toUnit(width) }"><div class="k-navigation-drawer__content"><slot /></div></nav>
</template>
