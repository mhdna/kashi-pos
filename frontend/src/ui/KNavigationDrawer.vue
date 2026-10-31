<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import { toUnit } from "./util";
defineProps<{ width?: string | number; permanent?: boolean; touchless?: boolean }>();

// The page content makes room for the drawer: <k-main> pads by --k-layout-left.
const el = ref<HTMLElement | null>(null);
onMounted(() => document.documentElement.style.setProperty("--k-layout-left", `${el.value?.offsetWidth ?? 0}px`));
onBeforeUnmount(() => document.documentElement.style.removeProperty("--k-layout-left"));
</script>
<template>
  <nav ref="el" class="k-navigation-drawer" :style="{ '--k-drawer-w': toUnit(width) }"><div class="k-navigation-drawer__content"><slot /></div></nav>
</template>
