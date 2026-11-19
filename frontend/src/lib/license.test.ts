import { describe, expect, it } from "vitest";
import golden from "./testdata/license_status.json";
import {
  LOCK_MESSAGE,
  activationErrorText,
  formatProductKey,
  licenseScreen,
  offlineWarningText,
  productKeyComplete,
} from "./license";

describe("licenseScreen", () => {
  it("reads the backend's real statuses (license_status.json from TestLicenseStatusGolden)", () => {
    expect(licenseScreen(golden.activate)).toBe("activate");
    expect(licenseScreen(golden.ok)).toBe("ok");
    expect(licenseScreen(golden.okOfflineWarning)).toBe("ok");
    expect(licenseScreen(golden.lockedOff)).toBe("locked");
    expect(licenseScreen(golden.lockedOffline)).toBe("locked");
    expect(licenseScreen(golden.licensingOff)).toBe("ok");
    expect(offlineWarningText(golden.okOfflineWarning.warnHours)).not.toBe("");
    expect(offlineWarningText(golden.ok.warnHours)).toBe("");
  });

  it("never locks for an unknown or missing status (an older backend)", () => {
    for (const s of [undefined, null, {}, { state: "" }, { state: "weird" }, { state: "OK" }]) {
      expect(licenseScreen(s as any)).toBe("ok");
    }
  });
});

describe("activationErrorText", () => {
  it("names only the mistakes the person can fix", () => {
    expect(activationErrorText(new Error("license:invalid_key"))).toMatch(/product key isn't right/);
    expect(activationErrorText("license:no_seats")).toMatch(/already used/);
    expect(activationErrorText("license:offline")).toMatch(/internet/);
    expect(activationErrorText("license:too_many_attempts")).toMatch(/Wait a minute/);
  });

  it("says the vague lock message for a turned-off customer and anything else", () => {
    for (const e of ["license:off", "license:server_error", "license:save_failed", "license:fingerprint", "boom", "", undefined, new Error("x")]) {
      expect(activationErrorText(e)).toBe(LOCK_MESSAGE);
    }
    expect(activationErrorText("license:off")).not.toMatch(/pay|turned|disabled|off/i);
  });
});

describe("formatProductKey", () => {
  it("formats as it is typed", () => {
    expect(formatProductKey("")).toBe("");
    expect(formatProductKey("a")).toBe("KASHI-A");
    expect(formatProductKey("abcde")).toBe("KASHI-ABCDE");
    expect(formatProductKey("abcdef")).toBe("KASHI-ABCDE-F");
    expect(formatProductKey("kashi-7q2mx-4hbka-zr0tn-9wcpe")).toBe("KASHI-7Q2MX-4HBKA-ZR0TN-9WCPE");
    expect(formatProductKey(" 7q2mx 4hbka zr0tn 9wcpe ")).toBe("KASHI-7Q2MX-4HBKA-ZR0TN-9WCPE");
    expect(formatProductKey("KASHI-7Q2MX-4HBKA-ZR0TN-9WCPE-EXTRA")).toBe("KASHI-7Q2MX-4HBKA-ZR0TN-9WCPE");
  });

  it("is stable and complete only at 20 characters, over random input", () => {
    const chars = "abcdefghjkmnpqrstvwxyz0123456789- _.KASHI";
    let seed = 11;
    const rand = () => ((seed = (seed * 1103515245 + 12345) % 2147483648) / 2147483648);
    for (let i = 0; i < 500; i++) {
      let s = "";
      const n = Math.floor(rand() * 40);
      for (let j = 0; j < n; j++) s += chars[Math.floor(rand() * chars.length)];
      const once = formatProductKey(s);
      expect(formatProductKey(once)).toBe(once);
      expect(once === "" || /^KASHI(-[0-9A-Z]{1,5}){1,4}$/.test(once)).toBe(true);
      const body = once.replace(/^KASHI-?/, "").replace(/-/g, "");
      expect(productKeyComplete(s)).toBe(body.length === 20);
    }
  });
});

describe("offlineWarningText", () => {
  it("says when, never why", () => {
    expect(offlineWarningText(0)).toBe("");
    expect(offlineWarningText(undefined)).toBe("");
    expect(offlineWarningText(-3)).toBe("");
    expect(offlineWarningText(5)).toMatch(/today/);
    expect(offlineWarningText(30)).toMatch(/within a day/);
    expect(offlineWarningText(47)).toMatch(/within a day/);
  });

  it("uses no long dashes or dots", () => {
    for (const text of [LOCK_MESSAGE, offlineWarningText(5), offlineWarningText(30), activationErrorText("license:invalid_key"), activationErrorText("license:no_seats")]) {
      expect(text).not.toMatch(/[—–·•]/);
    }
  });
});
