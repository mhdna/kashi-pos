<template>
  <div class="header-btn-wrap">
    <div class="header-btn-subtitle">Client</div>
    <v-btn
      size="small"
      color="teal-darken-1"
      variant="tonal"
      class="header-fixed-width-btn"
      :disabled="disabled"
      :class="{ 'missing-selection-pulse': pulseClient }"
      @click="dialog = true"
    >
      <v-icon icon="mdi-account-outline" size="16" class="me-1 flex-shrink-0" />
      <span class="header-fixed-width-btn-label">{{ clientSelected ? client.name.split(" ")[0] : "Client" }}</span>
    </v-btn>
  </div>
  <ClientDialog v-model="dialog" @select="setClient" />
</template>

<script setup lang="ts">
import { ref } from "vue";
import ClientDialog from "./ClientDialog.vue";
import { useClient } from "@/composables/useClient";

defineProps<{ disabled?: boolean }>();
const dialog = ref(false);
const { client, clientSelected, pulseClient, setClient } = useClient();
</script>

<style scoped>
.header-btn-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
}
.header-btn-subtitle {
  font-size: 0.8rem;
  line-height: 1.2;
  color: rgba(var(--v-theme-on-surface), 0.6);
  margin-bottom: 2px;
}
.header-fixed-width-btn {
  width: 108px;
}
.header-fixed-width-btn-label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.missing-selection-pulse {
  animation: missing-selection-pulse 700ms ease-in-out 2;
  border-radius: 12px;
}

@keyframes missing-selection-pulse {
  0%,
  100% {
    box-shadow: 0 0 0 0 rgba(var(--v-theme-warning), 0);
  }
  50% {
    box-shadow: 0 0 0 8px rgba(var(--v-theme-warning), 0.32);
  }
}
</style>
