<script setup lang="ts">
import { provide } from "vue";
import { btnDefaultsKey } from "./keys";
provide(btnDefaultsKey, { variant: "text" });
</script>
<template><div class="k-card-actions"><slot /></div></template>
