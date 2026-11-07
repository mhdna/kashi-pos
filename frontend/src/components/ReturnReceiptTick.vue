<template>
  <!-- An exchange's "Print receipt", under its summary. Only shown while
       receipts are on: with them off nothing prints anyway. -->
  <div class="return-receipt-tick" data-test="return-receipt-tick">
    <k-checkbox
      :model-value="modelValue"
      label="Print receipt"
      density="compact"
      hide-details
      color="primary"
      @update:model-value="emit('update:modelValue', Boolean($event))"
    />
  </div>
</template>

<script setup lang="ts">
defineProps<{ modelValue: boolean }>();
const emit = defineEmits<{ "update:modelValue": [value: boolean] }>();
</script>

<style scoped>
.return-receipt-tick {
  margin: 0 8px;
}
</style>
