import { fileURLToPath, URL } from "node:url";
import Vue from "@vitejs/plugin-vue";
/// <reference types="vitest/config" />
import { defineConfig } from "vite";

export default defineConfig({
  base: "./",
  plugins: [Vue()],
  test: { setupFiles: ["src/test/useEnglish.ts"] },
  resolve: {
    alias: { "@": fileURLToPath(new URL("src", import.meta.url)) },
    extensions: [".js", ".json", ".jsx", ".mjs", ".ts", ".tsx", ".vue"],
  },
  server: {
    port: 3006,
    fs: {
      allow: [".."], // or an absolute path to your monorepo root
    },
  },
});
