<template>
  <!-- Just the icon, no caption and no background. The wrapper leaves the
       room the buttons beside it use for their captions, so the icon sits on
       the same line as theirs. -->
  <div class="theme-toggle-wrap">
    <k-btn
      class="theme-toggle-btn"
      size="small"
      variant="text"
      color="blue-grey-darken-1"
      :loading="saving"
      :aria-label="`Switch to ${currentTheme === 'dark' ? 'light' : 'dark'} theme`"
      :title="`Switch to ${currentTheme === 'dark' ? 'light' : 'dark'} theme`"
      data-test="theme-toggle"
      @click="toggleTheme"
    >
      <k-icon :icon="currentTheme === 'dark' ? 'mdi-weather-sunny' : 'mdi-weather-night'" size="24" />
    </k-btn>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useTheme } from "@/ui";
import { SetTheme } from "../../wailsjs/go/main/App";

const theme = useTheme();

const currentTheme = theme.name;
const saving = ref(false);

// Switches at once, then saves; if saving fails it goes back.
async function toggleTheme() {
  if (saving.value) return;
  const previous = theme.name.value;
  const next = theme.name.value === "dark" ? "light" : "dark";
  theme.change(next);
  saving.value = true;
  try {
    await SetTheme(next);
  } catch {
    theme.change(previous);
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped>
.theme-toggle-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  /* The height of the captions over the Salesperson and Client buttons. */
  padding-top: calc(0.8rem * 1.2 + 2px);
}
.theme-toggle-btn {
  width: 44px;
  height: 35px;
  min-width: 0;
}
</style>
