export type MatchPart = { text: string; hit: boolean };

/**
 * Splits text into the pieces that match term and the pieces between them,
 * so a search result can colour what was searched for: "n24" in "N24-N245"
 * gives N24 (hit), "-" , N24 (hit), "5". Case-insensitive, like the
 * products search itself (a plain LIKE '%term%'). No term, or no match,
 * gives the whole text as one plain part.
 */
export function matchParts(text: string, term: string): MatchPart[] {
  const needle = term.trim().toLowerCase();
  if (!text || !needle) return text ? [{ text, hit: false }] : [];
  const haystack = text.toLowerCase();
  const parts: MatchPart[] = [];
  let from = 0;
  for (let at = haystack.indexOf(needle); at !== -1; at = haystack.indexOf(needle, at + needle.length)) {
    if (at > from) parts.push({ text: text.slice(from, at), hit: false });
    parts.push({ text: text.slice(at, at + needle.length), hit: true });
    from = at + needle.length;
  }
  if (from < text.length) parts.push({ text: text.slice(from), hit: false });
  return parts;
}
