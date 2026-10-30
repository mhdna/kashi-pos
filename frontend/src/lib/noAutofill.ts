const FIELDS = "input, textarea";

function turnOff(root: ParentNode) {
  if (root instanceof Element && root.matches(FIELDS)) {
    root.setAttribute("autocomplete", "off");
  }
  for (const field of root.querySelectorAll(FIELDS)) {
    field.setAttribute("autocomplete", "off");
  }
}

/**
 * Keep WebView2's "Saved info" list off the till's fields.
 *
 * WebView2 remembers what was typed into a field and offers it back in a
 * popup under it the next time, so a quantity or price box showed old
 * values ("0") over the cart. Wails gives no way to turn its autofill off,
 * so every input and textarea is marked autocomplete="off" here instead, as
 * soon as it reaches the page.
 */
export function preventAutofill() {
  turnOff(document);
  new MutationObserver((mutations) => {
    for (const mutation of mutations) {
      for (const node of mutation.addedNodes) {
        if (node instanceof Element) turnOff(node);
      }
    }
  }).observe(document.documentElement, { childList: true, subtree: true });
}
