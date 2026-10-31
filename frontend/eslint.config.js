import js from "@eslint/js";
import pluginVue from "eslint-plugin-vue";
import tseslint from "typescript-eslint";

export default [
  { ignores: ["dist/**", "wailsjs/**", "node_modules/**"] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  ...pluginVue.configs["flat/essential"],
  {
    files: ["**/*.vue"],
    languageOptions: { parserOptions: { parser: tseslint.parser } },
  },
  {
    rules: {
      "no-undef": "off", // TypeScript already reports unknown names
      "no-useless-assignment": "off",
      // Existing app code trips these; report them without failing.
      "no-empty": "warn",
      "no-irregular-whitespace": "warn",
      "@typescript-eslint/no-unused-vars": "warn",
      "vue/no-dupe-keys": "warn",
      "vue/multi-word-component-names": "off",
      "vue/valid-v-slot": "off", // data-table slots are named like #item.createdAt
      "@typescript-eslint/no-explicit-any": "off",
    },
  },
];
