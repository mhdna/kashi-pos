import { onBeforeUnmount, onMounted, ref, type Ref } from "vue";

// Tracks a scroll container's scrollTop and visible height (one passive scroll
// listener and one ResizeObserver) for the virtual lists.
export function useScrollWindow(el: Ref<HTMLElement | null>) {
  const scrollTop = ref(0);
  const height = ref(0);
  let observer: ResizeObserver | undefined;
  let frame = 0;

  function onScroll() {
    // Coalesce scroll events into one update per frame.
    if (frame) return;
    frame = requestAnimationFrame(() => {
      frame = 0;
      if (el.value) scrollTop.value = el.value.scrollTop;
    });
  }
  onMounted(() => {
    const node = el.value;
    if (!node) return;
    height.value = node.clientHeight;
    node.addEventListener("scroll", onScroll, { passive: true });
    if (typeof ResizeObserver !== "undefined") {
      observer = new ResizeObserver(() => {
        height.value = node.clientHeight;
      });
      observer.observe(node);
    }
  });
  onBeforeUnmount(() => {
    el.value?.removeEventListener("scroll", onScroll);
    observer?.disconnect();
    if (frame) cancelAnimationFrame(frame);
  });
  return { scrollTop, height };
}
