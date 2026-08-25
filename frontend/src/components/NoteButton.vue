<template>
  <div class="header-btn-wrap">
    <div class="header-btn-subtitle">Note</div>
    <v-btn
      size="small"
      variant="text"
      aria-label="Invoice note"
      class="header-fixed-width-btn note-btn"
      :class="{ 'note-has-content': hasNote }"
      @click="openNoteDialog"
    >
      <v-icon :icon="noteIcon" size="20" class="flex-shrink-0" />
    </v-btn>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";

const props = defineProps<{ note: string }>();
const emit = defineEmits<{ openDialog: [] }>();

const hasNote = computed(() => props.note.trim().length > 0);
const noteIcon = computed(() => hasNote.value ? "mdi-check-circle" : "mdi-note-text-outline");

function openNoteDialog() {
  emit("openDialog");
}
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
  height: 35px;
}

.note-btn {
  width: auto;
  height: 35px;
  color: rgba(var(--v-theme-on-surface), 0.6);
}

.note-btn.note-has-content {
  color: rgb(245, 127, 23);
}
</style>
