import { afterEach, describe, expect, it, vi } from "vitest";
import { closeTopOverlay, overlayDepth, pushOverlay } from "./overlay";

const esc = () => closeTopOverlay();

describe("overlay stack", () => {
  const releases: Array<() => void> = [];
  afterEach(() => {
    releases.splice(0).forEach((r) => r());
  });

  it("closes only the top overlay on Escape", () => {
    const bottom = vi.fn();
    const top = vi.fn();
    releases.push(pushOverlay(bottom), pushOverlay(top));
    esc();
    expect(top).toHaveBeenCalledTimes(1);
    expect(bottom).not.toHaveBeenCalled();
  });

  it("falls back to the one below once the top is released", () => {
    const bottom = vi.fn();
    const releaseBottom = pushOverlay(bottom);
    const releaseTop = pushOverlay(() => undefined);
    releaseTop();
    esc();
    expect(bottom).toHaveBeenCalledTimes(1);
    releaseBottom();
    expect(overlayDepth()).toBe(0);
  });
});
