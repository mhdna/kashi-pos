import { describe, expect, it, vi } from "vitest";
import { restoreSession, saveRoute } from "./resumeSession";

function deps(raw: any, loggedIn = false) {
  const calls: string[] = [];
  return {
    calls,
    d: {
      take: vi.fn(async () => raw),
      isLoggedIn: () => loggedIn,
      setUser: (u: any) => calls.push(`user:${u.role}`),
      go: async (r: string) => void calls.push(`go:${r}`),
    },
  };
}

describe("restoreSession", () => {
  it("signs in and goes to the saved screen", async () => {
    const { d, calls } = deps({ role: "cashier", route: "/invoices-list" });
    expect(await restoreSession(d)).toBe(true);
    expect(calls).toEqual(["user:cashier", "go:/invoices-list"]);
  });

  it("does nothing when there is nothing to resume", async () => {
    const { d, calls } = deps({});
    expect(await restoreSession(d)).toBe(false);
    expect(calls).toEqual([]);
  });

  it("leaves someone who signed in by hand alone", async () => {
    const { d, calls } = deps({ role: "admin", route: "/shifts" }, true);
    expect(await restoreSession(d)).toBe(false);
    expect(calls).toEqual([]);
  });

  it("never throws, whatever fails", async () => {
    const { d } = deps({ role: "admin" });
    d.take = vi.fn(async () => { throw new Error("no backend"); });
    expect(await restoreSession(d)).toBe(false);
    const { d: d2 } = deps({ role: "admin" });
    d2.go = async () => { throw new Error("nav failed"); };
    expect(await restoreSession(d2)).toBe(false);
  });
});

describe("saveRoute", () => {
  it("flushes the sale draft first, then saves the route", async () => {
    const order: string[] = [];
    await saveRoute({
      flushDraft: () => order.push("flush"),
      route: () => "/home",
      save: async (r) => void order.push(`save:${r}`),
    });
    expect(order).toEqual(["flush", "save:/home"]);
  });

  it("never throws", async () => {
    await expect(
      saveRoute({ flushDraft: () => {}, route: () => "/x", save: async () => { throw new Error("x"); } }),
    ).resolves.toBeUndefined();
  });
});
