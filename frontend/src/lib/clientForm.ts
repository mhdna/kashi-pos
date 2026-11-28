// The client dialog is one form: the phone number on the left, the name and
// "where did they hear about us?" on the right. As soon as the number is a
// whole valid one it is looked up: a known client is shown and picked with
// Continue; an unknown number is saved with the name (and an answer, when
// central has options).
import { canSaveNewClient, type ClientSourceOption } from "@/lib/clientSource";

// "none": the number isn't a whole valid one yet. "checking": being looked up.
// "found": a client has it. "new": nobody has it.
export type ClientLookup = "none" | "checking" | "found" | "new";

// Whether the form's main button does anything.
export function canSubmitClientForm(
  lookup: ClientLookup,
  name: string,
  options: ClientSourceOption[],
  picked: string,
): boolean {
  if (lookup === "found") return true;
  if (lookup !== "new") return false;
  return canSaveNewClient(name, options, picked);
}

// The main button's label: Continue picks the client the number found, Save
// adds a new one.
export function clientFormButton(lookup: ClientLookup): "Continue" | "Save" {
  return lookup === "found" ? "Continue" : "Save";
}
