import { describe, expect, it } from "vitest";
import { resumedSession, safeResumeRoute } from "./resume";

describe("safeResumeRoute", () => {
  it("keeps real in-app routes, query included", () => {
    for (const r of ["/home", "/invoices-list?code=SA/1", "/shifts", "/settings", "/clients#top"]) {
      expect(safeResumeRoute(r)).toBe(r);
    }
  });

  it("refuses anything that is not a plain in-app path", () => {
    const bad = [
      "", "   ", "home", "//evil.com", "https://evil.com", "javascript:alert(1)", "/a b", "/a\\b",
      "/", "/login", "/login?x=1", "/auth", "/auth/x", null, undefined, 5, {}, "/" + "x".repeat(600),
    ];
    for (const r of bad) expect(safeResumeRoute(r as any)).toBe("");
  });
});

describe("resumedSession", () => {
  it("restores the role and goes to the saved screen", () => {
    expect(resumedSession({ id: 0, username: "admin", role: "admin", route: "/shifts" })).toEqual({
      user: { id: 0, username: "admin", role: "admin" },
      route: "/shifts",
    });
  });

  it("falls back to /home when the saved screen is missing or unsafe", () => {
    for (const route of [undefined, "", "//x", "/login"]) {
      expect(resumedSession({ role: "cashier", route })?.route).toBe("/home");
    }
  });

  it("restores nothing without a known role", () => {
    for (const raw of [{}, null, undefined, { role: "" }, { role: "root" }, { role: "Admin" }, { route: "/home" }]) {
      expect(resumedSession(raw)).toBeNull();
    }
  });
});
