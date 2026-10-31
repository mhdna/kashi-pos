<script setup lang="ts">
import { provide } from "vue";
import { formKey } from "./keys";

const validators = new Set<() => boolean>();
provide(formKey, {
  register(validate) {
    validators.add(validate);
    return () => validators.delete(validate);
  },
});

// Runs every field's rules (showing their messages); { valid } like Vuetify's form.
async function validate() {
  let valid = true;
  for (const run of validators) if (!run()) valid = false;
  return { valid };
}
defineExpose({ validate });
</script>
<template><form novalidate><slot /></form></template>
