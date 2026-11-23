<template>
  <!-- Same shape as the Salesperson and Client buttons beside it: a small
       caption over a tonal button. -->
  <!-- Hidden for now (SHOW_THEME_TOGGLE below); set it to true to bring the button back. -->
  <div v-show="SHOW_THEME_TOGGLE" class="theme-toggle-wrap" data-test="theme-toggle-wrap">
    <div class="theme-toggle-subtitle">{{ t("Theme") }}</div>
    <k-btn
      class="theme-toggle-btn"
      size="small"
      variant="tonal"
      color="blue-grey-darken-1"
      :loading="saving"
      :aria-label="currentTheme === 'dark' ? t('Switch to light theme') : t('Switch to dark theme')"
      :title="currentTheme === 'dark' ? t('Switch to light theme') : t('Switch to dark theme')"
      data-test="theme-toggle"
      @click="toggleTheme"
    >
      <k-icon :icon="currentTheme === 'dark' ? 'mdi-weather-sunny' : 'mdi-weather-night'" size="18" />
    </k-btn>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useTheme } from "@/ui";
import { t } from "@/i18n";
import { SetTheme } from "../../wailsjs/go/main/App";

// The header's Theme button is hidden for now; the theme stays whatever it is.
const SHOW_THEME_TOGGLE = false;

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
}
.theme-toggle-subtitle {
  font-size: 0.8rem;
  line-height: 1.2;
  color: rgba(var(--k-theme-on-surface), 0.6);
  margin-bottom: 2px;
}
.theme-toggle-btn {
  width: 38px;
  min-width: 0;
  height: 35px;
}
</style>
