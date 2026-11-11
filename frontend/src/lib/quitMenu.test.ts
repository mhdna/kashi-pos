import { describe, expect, it } from "vitest";
import { QUIT_GROUPS, QUIT_OPTIONS } from "./quitMenu";

describe("quit menu", () => {
  it("groups the options by what they affect, in this order", () => {
    expect(QUIT_GROUPS.map((g) => g.title)).toEqual(["Shift", "This app", "This computer"]);
    expect(QUIT_GROUPS.map((g) => g.options.map((o) => o.action))).toEqual([
      ["close-shift", "logout"],
      ["reload-app", "close-app"],
      ["restart-pc", "shutdown-pc"],
    ]);
  });

  it("gives each group its own colour family", () => {
    expect(QUIT_GROUPS.map((g) => g.tone)).toEqual(["shift", "app", "computer"]);
  });

  it("names the app reload and the computer restart so they can't be confused", () => {
    const label = (a: string) => QUIT_OPTIONS.find((o) => o.action === a)!.label;
    expect(label("reload-app")).toBe("Reload App");
    expect(label("restart-pc")).toBe("Restart PC");
    expect(label("shutdown-pc")).toBe("Shut Down PC");
    // No option is just "Restart": it used to mean the computer.
    expect(QUIT_OPTIONS.map((o) => o.label)).not.toContain("Restart");
  });

  it("has unique actions, labels and a registered-looking icon for each", () => {
    expect(new Set(QUIT_OPTIONS.map((o) => o.action)).size).toBe(QUIT_OPTIONS.length);
    expect(new Set(QUIT_OPTIONS.map((o) => o.label)).size).toBe(QUIT_OPTIONS.length);
    for (const o of QUIT_OPTIONS) expect(o.icon).toMatch(/^mdi-/);
  });
});
