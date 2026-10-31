export interface Box {
  left: number;
  right: number;
  top: number;
  bottom: number;
}

// Where to put a floating box of `width` x `height` next to `anchor`.
// `location` is Vuetify's "<side> <align>": "bottom start", "bottom end",
// "bottom center", "top", "end"... `rtl` flips start/end. The result is kept
// inside the viewport, flipping above/below when the preferred side is cramped.
export function placeBox(
  anchor: Box,
  width: number,
  height: number,
  viewport: { width: number; height: number },
  location = "bottom start",
  gap = 0,
  rtl = false,
  margin = 8,
): { left: number; top: number } {
  const [sideRaw = "bottom", alignRaw] = location.split(" ");
  let side = sideRaw;
  const align = alignRaw ?? (side === "top" || side === "bottom" ? "center" : "start");
  const resolve = (v: string) => (rtl ? (v === "start" ? "end" : v === "end" ? "start" : v) : v);

  let left = 0;
  let top = 0;
  if (side === "top" || side === "bottom") {
    top = side === "bottom" ? anchor.bottom + gap : anchor.top - gap - height;
    const a = resolve(align);
    left = a === "start" ? anchor.left : a === "end" ? anchor.right - width : (anchor.left + anchor.right) / 2 - width / 2;
    // flip when it doesn't fit and the other side has more room
    const below = viewport.height - anchor.bottom;
    const above = anchor.top;
    if (side === "bottom" && top + height > viewport.height - margin && above > below) {
      top = anchor.top - gap - height;
      side = "top";
    } else if (side === "top" && top < margin && below > above) {
      top = anchor.bottom + gap;
      side = "bottom";
    }
  } else {
    const r = resolve(side);
    left = r === "end" ? anchor.right + gap : anchor.left - gap - width;
    top = align === "end" ? anchor.bottom - height : align === "center" ? (anchor.top + anchor.bottom) / 2 - height / 2 : anchor.top;
  }
  left = Math.max(margin, Math.min(left, viewport.width - width - margin));
  top = Math.max(margin, Math.min(top, viewport.height - height - margin));
  return { left, top };
}
