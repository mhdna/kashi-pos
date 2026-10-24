import type { Router } from "vue-router";

/**
 * Keep the till in one window.
 *
 * WebView2 opens a link in a new window on Ctrl/Shift/Alt-click or a middle
 * click, and that window is a second copy of the till. A modifier stuck down
 * over AnyDesk (a Mac's Caps Lock mapped to Ctrl, say) turned every tap on a
 * nav item into one. Router links also ignore modified clicks, so the tap
 * did nothing in the till itself. Here such a click on a link is stopped
 * before WebView2 sees it and, for an in-app link, followed as a plain
 * click would be.
 */
export function preventNewWindows(router: Router) {
  const onClick = (event: MouseEvent) => {
    const modified = event.ctrlKey || event.shiftKey || event.altKey || event.metaKey;
    if (!modified && event.button === 0) return;
    const link = (event.target as Element | null)?.closest?.("a[href]");
    if (!link) return;
    event.preventDefault();
    event.stopPropagation();
    const href = link.getAttribute("href") ?? "";
    if (event.button === 0 && href.startsWith("#/")) {
      void router.push(href.slice(1));
    }
  };
  window.addEventListener("click", onClick, true);
  window.addEventListener("auxclick", onClick, true);
  // Nothing in the till opens windows; a page that tried would get a copy.
  window.open = () => null;
}
