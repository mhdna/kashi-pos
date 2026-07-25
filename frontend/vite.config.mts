import { fileURLToPath, URL } from "node:url";
import Vue from "@vitejs/plugin-vue";
import { defineConfig } from "vite";
import Vuetify, { transformAssetUrls } from "vite-plugin-vuetify";

// import path from 'path'

export default defineConfig({
  base: "./",
  plugins: [
    Vue({ template: { transformAssetUrls } }),
    Vuetify({ autoImport: true, styles: { configFile: "src/styles/settings.scss" } }),
  ],
  resolve: {
    alias: { "@": fileURLToPath(new URL("src", import.meta.url)) },
    extensions: [".js", ".json", ".jsx", ".mjs", ".ts", ".tsx", ".vue"],
  },
  server: {
    port: 3006,
    fs: {
      allow: [".."], // or an absolute path to your monorepo root
    },
    // fs: {
    //   allow: [
    //     path.resolve(__dirname, 'node_modules'),
    //   ],
    // },
  },
});
