<script setup lang="ts">
defineProps<{ density?: string; hover?: boolean; fixedHeader?: boolean; height?: string | number }>();
</script>
<template>
  <div class="k-table" :class="[hover && 'k-table--hover', fixedHeader && 'k-table--fixed-header', density && density !== 'default' && `k-table--density-${density}`]" :style="{ height: typeof height === 'number' ? `${height}px` : height }">
    <div class="k-table__wrapper"><table><slot /></table></div>
  </div>
</template>
