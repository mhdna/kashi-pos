import { computed, ref } from "vue";

// The invoices list's load-more-as-you-scroll, for any list whose backend
// call takes (pageSize, pageIndex). Bind `onScroll` with
// @scroll.passive.capture on the element around a v-data-table-virtual: it
// fetches the next page when the table's own scroll gets within
// PRELOAD_DISTANCE_PX of the bottom.

const PRELOAD_DISTANCE_PX = 600;

export function useInfiniteList<T extends { id: number | string }>(
  fetchPage: (pageSize: number, pageIndex: number) => Promise<T[]>,
  pageSize = 30,
) {
  const items = ref<T[]>([]) as { value: T[] };
  const loading = ref(false);
  const initialized = ref(false);
  const endReached = ref(false);
  const hasMore = computed(() => !endReached.value);

  let pageIndex = 0;
  let generation = 0;
  let active: Promise<void> | null = null;
  let frame: number | null = null;
  const seen = new Set<number | string>();

  function loadMore(): Promise<void> {
    if (active || endReached.value) return active ?? Promise.resolve();
    const gen = generation;
    loading.value = true;
    active = (async () => {
      try {
        const page = (await fetchPage(pageSize, pageIndex)) ?? [];
        if (gen !== generation) return;
        const fresh = page.filter((item) => {
          if (seen.has(item.id)) return false;
          seen.add(item.id);
          return true;
        });
        items.value = [...items.value, ...fresh];
        endReached.value = page.length < pageSize;
        pageIndex++;
      } catch {
        if (gen === generation) endReached.value = true;
      } finally {
        if (gen === generation) {
          initialized.value = true;
          loading.value = false;
        }
        active = null;
      }
    })();
    return active;
  }

  async function refresh() {
    const previous = active;
    generation++;
    items.value = [];
    seen.clear();
    pageIndex = 0;
    endReached.value = false;
    initialized.value = false;
    active = null;
    if (previous) await previous.catch(() => {});
    await loadMore();
  }

  function onScroll(event: Event) {
    if (frame !== null || loading.value || endReached.value) return;
    const target = event.target;
    if (!(target instanceof HTMLElement) || !target.classList.contains("v-table__wrapper")) return;
    frame = requestAnimationFrame(() => {
      frame = null;
      if (target.scrollHeight - target.scrollTop - target.clientHeight <= PRELOAD_DISTANCE_PX) void loadMore();
    });
  }

  return { items, loading, initialized, hasMore, loadMore, refresh, onScroll };
}
