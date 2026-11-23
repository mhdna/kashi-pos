// Applies a watcher's click, typing and key press to this till's own page.
// Only used while kashi allows control (liveView.ts). Targets are rrweb node
// ids, so a message for a node that no longer exists is ignored.

export interface NodeMirror {
  getNode(id: number): Node | null;
}

export type ControlMessage =
  | { t: "click"; id: number }
  | { t: "input"; id: number; v: string }
  | { t: "key"; key: string };

const NAMED_KEYS = new Set(["Enter", "Escape", "Tab", "Backspace", "Delete", "ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight"]);

export function parseControl(raw: string): ControlMessage | null {
  let m: any;
  try {
    m = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!m || typeof m !== "object") return null;
  if (m.t === "click" && Number.isInteger(m.id) && m.id > 0) return { t: "click", id: m.id };
  if (m.t === "input" && Number.isInteger(m.id) && m.id > 0 && typeof m.v === "string") {
    return { t: "input", id: m.id, v: m.v.slice(0, 2000) };
  }
  if (m.t === "key" && typeof m.key === "string" && (NAMED_KEYS.has(m.key) || [...m.key].length === 1)) {
    return { t: "key", key: m.key };
  }
  return null;
}

/** The watcher's clock stamp on a control message (ts), echoed back as an ack so the page can show the delay. */
export function parseStamp(raw: string): number | null {
  try {
    const ts = JSON.parse(raw)?.ts;
    return Number.isSafeInteger(ts) && ts > 0 ? ts : null;
  } catch {
    return null;
  }
}

const asElement = (node: Node | null): HTMLElement | null => {
  if (!node) return null;
  if (node.nodeType === 1) return node as HTMLElement;
  return node.parentElement;
};

function isTextField(el: Element | null): el is HTMLInputElement | HTMLTextAreaElement {
  if (!el) return false;
  if (el instanceof HTMLTextAreaElement) return true;
  if (!(el instanceof HTMLInputElement)) return false;
  return !["checkbox", "radio", "button", "submit", "file", "range", "color"].includes(el.type);
}

/** Sets a field's value the way typing would, so Vue and friends see an input event. */
function setValue(el: HTMLInputElement | HTMLTextAreaElement, value: string) {
  const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(proto, "value")?.set;
  if (setter) setter.call(el, value);
  else el.value = value;
  el.dispatchEvent(new Event("input", { bubbles: true }));
}

function focusables(doc: Document): HTMLElement[] {
  return [...doc.querySelectorAll<HTMLElement>('input, textarea, select, button, a[href], [tabindex]:not([tabindex="-1"])')].filter(
    (e) => !(e as HTMLInputElement).disabled && e.tabIndex >= 0,
  );
}

function pressKey(doc: Document, key: string): boolean {
  const active = doc.activeElement as HTMLElement | null;
  const target = active && active !== doc.body ? active : doc.body;
  const init = { key, bubbles: true, cancelable: true };
  const notCancelled = target.dispatchEvent(new KeyboardEvent("keydown", init));
  if (notCancelled) {
    if (isTextField(target)) {
      const start = target.selectionStart ?? target.value.length;
      const end = target.selectionEnd ?? start;
      if ([...key].length === 1) {
        setValue(target, target.value.slice(0, start) + key + target.value.slice(end));
        target.setSelectionRange?.(start + key.length, start + key.length);
      } else if (key === "Backspace") {
        const from = start === end ? Math.max(0, start - 1) : start;
        setValue(target, target.value.slice(0, from) + target.value.slice(end));
        target.setSelectionRange?.(from, from);
      } else if (key === "Delete") {
        const to = start === end ? Math.min(target.value.length, end + 1) : end;
        setValue(target, target.value.slice(0, start) + target.value.slice(to));
        target.setSelectionRange?.(start, start);
      }
    }
    if (key === "Enter" && target instanceof HTMLButtonElement) target.click();
    if (key === "Tab") {
      const list = focusables(doc);
      const at = list.indexOf(target);
      list[(at + 1) % list.length]?.focus();
    }
  }
  target.dispatchEvent(new KeyboardEvent("keyup", init));
  return true;
}

/** Applies one message. Returns whether it did anything. */
export function applyControl(message: ControlMessage, mirror: NodeMirror, doc: Document = document): boolean {
  if (message.t === "key") return pressKey(doc, message.key);
  const el = asElement(mirror.getNode(message.id));
  if (!el || !el.isConnected) return false;
  if (message.t === "click") {
    el.focus?.();
    el.click();
    return true;
  }
  if (isTextField(el)) {
    el.focus();
    setValue(el, message.v);
    el.dispatchEvent(new Event("change", { bubbles: true }));
    return true;
  }
  return false;
}
