<template>
  <v-card class="open-shift-card pa-4" color="green-lighten-4">
    <div class="d-flex align-center justify-space-between">
      <div>
        <div class="text-h6 font-weight-bold">Open Shift #{{ openShift?.id ?? "—" }}</div>
        <div class="text-body-2">Opened: {{ openShift?.createdAt ?? "—" }}</div>
      </div>
      <v-btn variant="elevated" color="white" :disabled="!openShift" @click="$emit('close')">
        Close Shift
      </v-btn>
    </div>
  </v-card>
</template>

<script setup lang="ts">
import type { Shift } from "@/types/shifts";

defineProps<{ openShift: Shift | null }>();
defineEmits<{ close: [] }>();
</script>

<style scoped>
.open-shift-card {
  flex: 0 0 auto;
}
</style>
