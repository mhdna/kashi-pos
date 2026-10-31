export interface VirtualRange {
  start: number; // first rendered index
  end: number; // one past the last rendered index
  padTop: number; // px of empty space above the rendered rows
  padBottom: number; // px of empty space below them
}

// Which fixed-height rows to render for the current scroll position, plus
// the spacer heights that keep the scrollbar the size of the full list.
export function visibleRange(scrollTop: number, viewportHeight: number, itemHeight: number, count: number, overscan = 6): VirtualRange {
  if (count <= 0 || itemHeight <= 0) return { start: 0, end: 0, padTop: 0, padBottom: 0 };
  // Before the first measurement there is no height to go by: render a screenful.
  const height = viewportHeight > 0 ? viewportHeight : itemHeight * 30;
  const first = Math.floor(Math.max(0, scrollTop) / itemHeight);
  const last = Math.ceil((Math.max(0, scrollTop) + height) / itemHeight);
  const start = Math.max(0, Math.min(count, first - overscan));
  const end = Math.max(start, Math.min(count, last + overscan));
  return { start, end, padTop: start * itemHeight, padBottom: (count - end) * itemHeight };
}

export type SortDirection = "asc" | "desc";
export interface SortState {
  key: string;
  direction: SortDirection;
}

// Header click cycle: ascending, descending, then back to unsorted.
export function nextSort(current: SortState | null, key: string): SortState | null {
  if (!current || current.key !== key) return { key, direction: "asc" };
  if (current.direction === "asc") return { key, direction: "desc" };
  return null;
}

export function sortItems<T>(items: readonly T[], sort: SortState | null): readonly T[] {
  if (!sort) return items;
  const sign = sort.direction === "asc" ? 1 : -1;
  const read = (item: T) => (item as Record<string, unknown>)[sort.key];
  return [...items].sort((a, b) => {
    const x = read(a);
    const y = read(b);
    if (x === y) return 0;
    if (x === undefined || x === null) return 1;
    if (y === undefined || y === null) return -1;
    if (typeof x === "number" && typeof y === "number") return (x - y) * sign;
    return String(x).localeCompare(String(y), undefined, { numeric: true, sensitivity: "base" }) * sign;
  });
}
