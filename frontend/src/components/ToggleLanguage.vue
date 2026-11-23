<template>
  <!-- Same shape as the Salesperson and Client buttons beside it: a small
       caption over a tonal button showing the language it switches to. -->
  <div class="d-flex flex-column align-center" data-test="language-toggle-wrap">
    <div style="font-size: var(--kashi-font-size-xs)">{{ t("Language") }}</div>
    <k-btn
      size="small"
      variant="tonal"
      color="blue-grey-darken-1"
      :aria-label="locale === 'fr' ? 'English' : 'Français'"
      data-test="language-toggle"
      @click="setLocale(locale === 'fr' ? 'en' : 'fr')"
    >
      {{ locale === "fr" ? "EN" : "FR" }}
    </k-btn>
  </div>
</template>

<script setup lang="ts">
import { locale, setLocale, t } from "@/i18n";
</script>
