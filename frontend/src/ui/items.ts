export interface Item {
  title: string;
  value: unknown;
  raw: unknown;
}

type Key = string | ((item: any) => unknown) | undefined;

function read(item: any, key: Key, fallback: unknown): unknown {
  if (typeof key === "function") return key(item);
  if (key !== undefined && item !== null && typeof item === "object" && key in item) return item[key];
  return fallback;
}

// Same item rules as Vuetify's select: primitives are their own title and
// value; objects use item-title / item-value (default "title" / "value"), and
// fall back to the object itself.
export function normaliseItems(items: readonly unknown[] | undefined, itemTitle?: Key, itemValue?: Key): Item[] {
  return (items ?? []).map((raw) => {
    if (raw === null || typeof raw !== "object") return { title: String(raw), value: raw, raw };
    const value = read(raw, itemValue ?? "value", raw);
    const title = read(raw, itemTitle ?? "title", value);
    return { title: title === undefined || title === null ? "" : String(title), value, raw };
  });
}

// Next selection after clicking `value` (multiple: toggle; single: replace).
export function toggleSelection(current: unknown, value: unknown, multiple: boolean): unknown {
  if (!multiple) return value;
  const list = Array.isArray(current) ? current : [];
  return list.includes(value) ? list.filter((v) => v !== value) : [...list, value];
}

export function isSelected(current: unknown, value: unknown, multiple: boolean): boolean {
  return multiple ? Array.isArray(current) && current.includes(value) : current === value;
}
