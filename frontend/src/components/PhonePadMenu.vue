<template>
  <!-- A phone keypad that pops up under its field, like the quantity and
       price pads (NumberPadMenu), so the form under it keeps its own
       keyboard. The keys go to the field through `press`; the field keeps
       the number and the caret. -->
  <k-menu v-model="open" :close-on-content-click="false" location="bottom center" :gap="6">
    <template #activator="{ props: activatorProps }">
      <slot name="activator" :props="activatorProps" />
    </template>

    <k-card class="phone-pad-menu pa-2" rounded="lg" data-test="phone-pad-menu">
      <div v-if="title" class="phone-pad-menu-heading">
        <span class="phone-pad-menu-title">{{ title }}</span>
      </div>
      <div class="phone-pad-menu-display" data-test="phone-pad-menu-display">{{ display || "0" }}</div>
      <!-- mousedown.prevent keeps the field focused, with its caret, while a
           key is tapped. -->
      <div class="phone-pad-menu-grid">
        <k-btn
          v-for="key in keys"
          :key="key.value"
          class="phone-pad-menu-key"
          size="small"
          variant="tonal"
          :aria-label="key.label"
          :data-test="`phone-pad-menu-key-${key.value}`"
          @mousedown.prevent
          @click="press(key.value)"
        >
          <k-icon v-if="key.icon" :icon="key.icon" size="20" />
          <template v-else>{{ key.text }}</template>
        </k-btn>
      </div>
      <k-btn
        block
        class="phone-pad-menu-done mt-2"
        color="primary"
        size="small"
        variant="flat"
        data-test="phone-pad-menu-done"
        @click="open = false"
      >
        {{ t("Done") }}
      </k-btn>
    </k-card>
  </k-menu>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { t } from "@/i18n";
import { playClickSound, playDtmfSound } from "@/lib/sound";

defineProps<{ title?: string; display?: string }>();
const emit = defineEmits<{ press: [key: string] }>();
const open = defineModel<boolean>("open", { default: false });

type Key = { value: string; label: string; text?: string; icon?: string };
const digit = (d: string): Key => ({ value: d, label: d, text: d });
// A computed so the labels follow the till language when it is toggled.
const keys = computed<Key[]>(() => [
  ...["1", "2", "3", "4", "5", "6", "7", "8", "9"].map(digit),
  { value: "clear", label: t("Clear phone number"), text: "C" },
  digit("0"),
  { value: "back", label: t("Delete digit"), icon: "mdi-backspace-outline" },
]);

function press(key: string) {
  if (/^\d$/.test(key)) playDtmfSound(key);
  else playClickSound();
  emit("press", key);
}
</script>

<style scoped>
/* Same card, heading, keys and Done as NumberPadMenu, a little wider for a
   whole phone number. */
.phone-pad-menu {
  width: 240px;
}
.phone-pad-menu-heading {
  padding: 2px 4px 0;
  text-align: center;
}
.phone-pad-menu-title {
  font-size: 0.6875rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgba(var(--k-theme-on-surface), 0.6);
}
.phone-pad-menu-display {
  font-size: var(--kashi-font-size-xl);
  font-variant-numeric: tabular-nums;
  font-weight: 700;
  min-height: 34px;
  overflow-wrap: anywhere;
  padding: 8px 0;
  text-align: center;
}
.phone-pad-menu-grid {
  display: grid;
  gap: 4px;
  grid-template-columns: repeat(3, 1fr);
}
.phone-pad-menu-key {
  font-size: var(--kashi-font-size-xl);
  height: 48px;
  min-width: 0;
  padding: 0;
}
.phone-pad-menu-done {
  height: 42px !important;
}
</style>
