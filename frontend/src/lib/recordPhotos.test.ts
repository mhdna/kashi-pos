import { readFileSync } from "node:fs";
import { describe, expect, it, vi } from "vitest";

vi.mock("../../wailsjs/runtime/runtime", () => ({ EventsOn: vi.fn(() => () => {}) }));

import { transferPhotoRef, usePhotoVersions } from "./recordPhotos";

const golden: { clientRef: string; transferID: number; ref: string }[] = JSON.parse(
  readFileSync(new URL("./testdata/record_photo_refs.json", import.meta.url), "utf8"),
);

describe("transferPhotoRef", () => {
  // Written by the Go side (TestRecordPhotoRefsGolden): the page must name a
  // transfer exactly as the till does in "record_photos_changed", or a
  // changed photo would never redraw.
  it("matches the till's names", () => {
    expect(golden.length).toBeGreaterThan(0);
    for (const c of golden) {
      expect(transferPhotoRef(c.clientRef, c.transferID)).toBe(c.ref);
    }
  });
});

describe("usePhotoVersions", () => {
  it("starts every record at version 0", () => {
    const { cacheKey } = usePhotoVersions();
    expect(cacheKey("expense-thumb", "exp-1")).toBe("expense-thumb-exp-1-0");
  });

  it("bumps one record without touching the others", () => {
    const { cacheKey, bump } = usePhotoVersions();
    bump("exp-1");
    bump("exp-1");
    bump("exp-2");
    expect(cacheKey("expense-thumb", "exp-1")).toBe("expense-thumb-exp-1-2");
    expect(cacheKey("expense-thumb", "exp-2")).toBe("expense-thumb-exp-2-1");
    expect(cacheKey("expense-thumb", "exp-3")).toBe("expense-thumb-exp-3-0");
  });

  it("ignores an empty ref", () => {
    const { versions, bump } = usePhotoVersions();
    bump("");
    expect(versions.value).toEqual({});
  });

  it("gives every key a new value on each bump (randomized)", () => {
    const { cacheKey, bump } = usePhotoVersions();
    const seen = new Set<string>();
    const refs = ["a", "b", "transfer-5", "transfer-55"];
    for (const r of refs) seen.add(cacheKey("t", r));
    for (let i = 0; i < 300; i++) {
      const r = refs[Math.floor(Math.random() * refs.length)]!;
      bump(r);
      const key = cacheKey("t", r);
      expect(seen.has(key)).toBe(false);
      seen.add(key);
    }
  });
});
