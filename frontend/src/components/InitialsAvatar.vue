<template>
  <v-avatar :color="props.color" size="60">
    <span style="letter-spacing: 1px; font-size: 25px;">{{ props.firstName[0] }}{{ props.lastName[0] }}</span>
  </v-avatar>
</template>

<script setup lang="ts">
  const props = defineProps<{
    color: string
    firstName: string
    lastName: string
  }>()
</script>
