import { ref, watch, type Ref } from "vue";

/**
 * Whether products search results show in columns, several to a row. Columns
 * are the default; a choice made with the toggle is remembered, kept apart
 * for code-only rows and named rows.
 */
export function useProductGridMode(namesHidden: Ref<boolean>) {
  const gridMode = ref(false);
  const key = () => `kashi-product-search-grid-${namesHidden.value ? "codes" : "named"}`;
  watch(
    namesHidden,
    () => {
      const saved = localStorage.getItem(key());
      gridMode.value = saved === null ? true : saved === "true";
    },
    { immediate: true },
  );
  watch(gridMode, (on) => localStorage.setItem(key(), String(on)));
  return gridMode;
}
