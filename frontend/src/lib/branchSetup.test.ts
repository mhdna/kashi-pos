import { describe, expect, it } from "vitest";
import { canSubmit, connectionHint, connectionMode, createBranchArgs, normalizeBranchCode, setupErrors } from "./branchSetup";

const form = (over = {}) => ({ name: "Centro", code: "CN", apiKey: "", centralApiUrl: "", ...over });

describe("normalizeBranchCode", () => {
  it("upper-cases and keeps only letters and digits", () => {
    expect(normalizeBranchCode("cn")).toBe("CN");
    expect(normalizeBranchCode(" c-n 2 ")).toBe("CN2");
    expect(normalizeBranchCode("çà!")).toBe("");
  });
  it("randomized: the result is always [A-Z0-9]* and idempotent", () => {
    const chars = "abcXYZ019 -_!é.";
    for (let n = 0; n < 500; n++) {
      let s = "";
      for (let i = 0; i < n % 12; i++) s += chars[(n * 7 + i * 13) % chars.length];
      const out = normalizeBranchCode(s);
      expect(out).toMatch(/^[A-Z0-9]*$/);
      expect(normalizeBranchCode(out)).toBe(out);
    }
  });
});

describe("connectionMode", () => {
  it("key and server: central; neither: standalone; one: which is missing", () => {
    expect(connectionMode(form({ apiKey: "k", centralApiUrl: "u" }))).toBe("central");
    expect(connectionMode(form())).toBe("standalone");
    expect(connectionMode(form({ apiKey: "k" }))).toBe("missing-url");
    expect(connectionMode(form({ centralApiUrl: "u" }))).toBe("missing-key");
    expect(connectionMode(form({ apiKey: "  ", centralApiUrl: " " }))).toBe("standalone");
  });
  it("every mode has a hint; the half-filled ones warn", () => {
    expect(connectionHint("central").tone).toBe("ok");
    expect(connectionHint("standalone").tone).toBe("info");
    expect(connectionHint("missing-url").tone).toBe("warn");
    expect(connectionHint("missing-key").tone).toBe("warn");
  });
});

describe("setupErrors / canSubmit", () => {
  it("name and code are required; central is optional", () => {
    expect(canSubmit(form())).toBe(true);
    expect(setupErrors(form({ name: " " })).name).toBeTruthy();
    expect(setupErrors(form({ code: "--" })).code).toBeTruthy();
    expect(canSubmit(form({ name: "" }))).toBe(false);
  });
});

describe("createBranchArgs", () => {
  it("trims everything and normalises the code", () => {
    expect(createBranchArgs({ name: " Centro ", code: "cn ", apiKey: " key ", centralApiUrl: " cloud.example.com/api " }))
      .toEqual(["Centro", "CN", "key", "cloud.example.com/api"]);
  });
});
