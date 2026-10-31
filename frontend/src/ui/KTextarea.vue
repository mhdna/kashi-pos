<script setup lang="ts">
import { ref } from "vue";
import KTextField from "./KTextField.vue";

const inner = ref<InstanceType<typeof KTextField> | null>(null);
defineExpose({
  focus: (options?: FocusOptions) => inner.value?.focus(options),
  blur: () => inner.value?.blur(),
  select: () => inner.value?.select(),
  validate: () => inner.value?.validate(),
});
</script>
<template><KTextField ref="inner" multiline /></template>
