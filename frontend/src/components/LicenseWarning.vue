<script setup lang="ts">
import { computed, ref } from "vue";
import { offlineWarningText } from "@/lib/license";

// The till hasn't reached central for a while and will stop starting soon.
const props = defineProps<{ warnHours?: number }>();
const dismissed = ref(false);
const text = computed(() => offlineWarningText(props.warnHours));
const show = computed({
  get: () => !dismissed.value && text.value !== "",
  set: (v) => { if (!v) dismissed.value = true; },
});
</script>

<template>
  <k-snackbar v-model="show" :timeout="-1" location="top" color="warning" data-test="license-warning">
    <div class="d-flex align-center">
      <k-icon icon="mdi-wifi-off" class="me-3" />
      <span>{{ text }}</span>
    </div>
    <template #actions>
      <k-btn variant="text" @click="dismissed = true">Later</k-btn>
    </template>
  </k-snackbar>
</template>
