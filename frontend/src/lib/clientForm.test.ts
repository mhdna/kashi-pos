import { describe, expect, it } from "vitest";
import { canSubmitClientForm, clientFormButton, type ClientLookup } from "./clientForm";
import fixture from "@/lib/testdata/client_sources.json";

const lookups: ClientLookup[] = ["none", "checking", "found", "new"];
const names = ["", "   ", "Maya", " ali "];

describe("canSubmitClientForm", () => {
  it("a found client continues whatever the name and answer say", () => {
    for (const name of names) for (const picked of ["", "x", fixture[0]!.uid]) {
      expect(canSubmitClientForm("found", name, fixture, picked)).toBe(true);
      expect(canSubmitClientForm("found", name, [], picked)).toBe(true);
    }
  });

  it("nothing submits before the number is whole and looked up", () => {
    for (const lookup of ["none", "checking"] as const) for (const name of names) {
      expect(canSubmitClientForm(lookup, name, fixture, fixture[1]!.uid)).toBe(false);
      expect(canSubmitClientForm(lookup, name, [], "")).toBe(false);
    }
  });

  it("a new number needs a name, and an answer when there are options", () => {
    expect(canSubmitClientForm("new", "", fixture, fixture[0]!.uid)).toBe(false);
    expect(canSubmitClientForm("new", "  ", [], "")).toBe(false);
    expect(canSubmitClientForm("new", "Maya", fixture, "")).toBe(false);
    expect(canSubmitClientForm("new", "Maya", fixture, "gone")).toBe(false);
    expect(canSubmitClientForm("new", "Maya", fixture, fixture[2]!.uid)).toBe(true);
    expect(canSubmitClientForm("new", "Maya", [], "")).toBe(true);
  });

  it("every combination: only found, or new with a name and a valid answer", () => {
    for (const lookup of lookups) for (const name of names) for (const opts of [fixture, []]) for (const picked of ["", "gone", ...fixture.map((o) => o.uid)]) {
      const answered = opts.length === 0 || opts.some((o) => o.uid === picked);
      const want = lookup === "found" || (lookup === "new" && name.trim() !== "" && answered);
      expect(canSubmitClientForm(lookup, name, opts, picked), `${lookup}/${name}/${opts.length}/${picked}`).toBe(want);
    }
  });
});

describe("clientFormButton", () => {
  it("Continue for a found client, Save otherwise", () => {
    expect(lookups.map(clientFormButton)).toEqual(["Save", "Save", "Continue", "Save"]);
  });
});
