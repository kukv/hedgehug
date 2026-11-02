import { quoteArg } from "../../core/cli";

export const NO_CANCELED = "-status:canceled";

// What the view fixes in the query: a workspace by name, or a project.
export type QueryBase =
  | { kind: "workspace"; name: string }
  | { kind: "project"; ulid: string; id: string };

// splitTerms cuts q at white space outside double quotes, as the query parser
// does; the quotes stay in the terms.
export function splitTerms(q: string): string[] {
  const out: string[] = [];
  let cur = "";
  let quoted = false;
  for (const ch of q) {
    if (ch === '"') {
      quoted = !quoted;
      cur += ch;
    } else if (/\s/.test(ch) && !quoted) {
      if (cur) out.push(cur);
      cur = "";
    } else {
      cur += ch;
    }
  }
  if (cur) out.push(cur);
  return out;
}

// baseQuery writes the base for the API (the project by ULID) or for the
// command line shown to the user (the project by ID).
export function baseQuery(b: QueryBase, form: "api" | "cli"): string {
  if (b.kind === "workspace") return `workspace:${quoteArg(b.name)}`;
  return `project:${form === "api" ? b.ulid : b.id}`;
}

// combine puts the base before the user's input. Every term of a query is
// ANDed, so joining them narrows the base and never leaves it.
export function combine(base: string, user: string): string {
  const u = user.trim();
  return u ? `${base} ${u}` : base;
}

export const hasTerm = (user: string, term: string) => splitTerms(user).includes(term);

export const withoutTerm = (user: string, term: string) =>
  splitTerms(user)
    .filter((t) => t !== term)
    .join(" ");

export function toggleTerm(user: string, term: string): string {
  return hasTerm(user, term) ? withoutTerm(user, term) : [...splitTerms(user), term].join(" ");
}
