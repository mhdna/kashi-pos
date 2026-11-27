import { describe, expect, it } from "vitest";
import fixture from "./testdata/client_sources.json";
import { canSaveNewClient, sourceOptions, sourceToSend, toggleSource } from "./clientSource";

const options = sourceOptions(fixture);

describe("client source", () => {
  it("the till's real list: central's starting options, in order", () => {
    expect(options.map((o) => o.name)).toEqual(["Social media", "SMS", "A friend", "Walking by", "Other"]);
  });

  it("drops blanks and repeats, keeps the order", () => {
    expect(
      sourceOptions([
        { uid: "b", name: "SMS" },
        { uid: " ", name: "No uid" },
        { uid: "c", name: "  " },
        { uid: "b", name: "SMS again" },
        { uid: "a", name: " Friend " },
      ]),
    ).toEqual([
      { uid: "b", name: "SMS" },
      { uid: "a", name: "Friend" },
    ]);
    expect(sourceOptions(null)).toEqual([]);
    expect(sourceOptions(undefined)).toEqual([]);
  });

  it("tapping picks, tapping again clears, tapping another moves", () => {
    expect(toggleSource("", "a")).toBe("a");
    expect(toggleSource("a", "a")).toBe("");
    expect(toggleSource("a", "b")).toBe("b");
  });

  it("only an option on the list is sent", () => {
    expect(sourceToSend(options, options[2]!.uid)).toBe(options[2]!.uid);
    expect(sourceToSend(options, "gone")).toBe("");
    expect(sourceToSend(options, "")).toBe("");
    expect(sourceToSend([], "x")).toBe("");
  });

  it("save needs a name and an answer, unless there is nothing to ask", () => {
    expect(canSaveNewClient("", options, options[0]!.uid)).toBe(false);
    expect(canSaveNewClient("  ", options, options[0]!.uid)).toBe(false);
    expect(canSaveNewClient("Maya", options, "")).toBe(false);
    expect(canSaveNewClient("Maya", options, "gone")).toBe(false);
    expect(canSaveNewClient("Maya", options, options[0]!.uid)).toBe(true);
    expect(canSaveNewClient("Maya", [], "")).toBe(true);
    expect(canSaveNewClient("", [], "")).toBe(false);
  });

  it("any taps on any list: save is allowed exactly when a listed option is picked", () => {
    let seed = 3;
    const rand = (n: number) => {
      seed = (seed * 1103515245 + 12345) % 2147483648;
      return seed % n;
    };
    for (let run = 0; run < 500; run++) {
      const list = sourceOptions(Array.from({ length: rand(6) }, (_, i) => ({ uid: `u${rand(8)}`, name: `N${i}` })));
      let picked = "";
      const taps = rand(6);
      for (let k = 0; k < taps; k++) picked = toggleSource(picked, `u${rand(9)}`);
      const listed = list.some((o) => o.uid === picked);
      expect(canSaveNewClient("Name", list, picked)).toBe(list.length === 0 || listed);
      expect(sourceToSend(list, picked)).toBe(listed ? picked : "");
    }
  });
});
