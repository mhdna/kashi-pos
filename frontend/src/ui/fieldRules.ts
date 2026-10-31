export type Rule = (value: unknown) => boolean | string | Promise<boolean | string>;

// First failing rule's message, or "" when everything passes (sync rules only).
export function firstRuleError(rules: Rule[] | undefined, value: unknown): string {
  for (const rule of rules ?? []) {
    const result = rule(value);
    if (typeof result === "string") return result;
    if (result === false) return " ";
  }
  return "";
}

export function normaliseMessages(messages: string | string[] | undefined): string[] {
  if (!messages) return [];
  return (Array.isArray(messages) ? messages : [messages]).filter(Boolean);
}
