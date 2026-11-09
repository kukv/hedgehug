import { CATEGORIES } from "../../core/categories";
import { KINDS } from "../../core/kinds";
import { BUILTIN_PROPS, type SchemaView } from "../../core/schema";
import { splitTerms } from "../tree/filter";

// "もしかして": the term of a query the server refused, written again with the name it is
// closest to. The server tells the cause only in English in the detail of query.invalid
// ("term 1 (status:progres): unknown status or category "progres""), so that is what is read.

// The keys the query reads on its own (internal/app/storage/query/eval.go, sort.go, mount/query.go).
const QUERY_KEYS = [
  "status",
  "kind",
  "label",
  "assignee",
  "watching",
  "mentions",
  "parent",
  "project",
  "workspace",
  "blocked_by",
  "blocks",
  "causes",
  "caused_by",
  "relates",
  "related_by",
  "duplicates",
  "duplicated_by",
  "clones",
  "cloned_by",
  "is",
  "has",
  "store",
  "sort",
  "group",
  // The built-in properties of a task; the labels are `label`, above.
  ...BUILTIN_PROPS.filter((b) => b.name !== "labels").map((b) => b.name),
];
const MAX_DISTANCE = 2;
// The shortest word that is taken for the part of a name.
const MIN_CONTAINED = 3;

type Problem = {
  raw: string; // the term as it was typed
  what: "status" | "kind" | "key" | "value";
  word: string; // the name that is wrong
  options?: string[]; // the names the server lists (an enum)
};

const HEAD = /^term \d+ \((.*?)\): (.*)$/s;
const NAMED: [RegExp, "status" | "kind" | "key"][] = [
  [/^unknown status or category "(.*)"$/s, "status"],
  [/^unknown kind "(.*)"$/s, "kind"],
  [/^unknown key "(.*)"$/s, "key"],
];
const ENUM = /^"(.*)" is not one of \[(.*)\]$/s;

function problemOf(error: unknown): Problem | null {
  if (typeof error !== "object" || error === null) return null;
  const e = error as { type?: unknown; detail?: unknown };
  if (e.type !== "query.invalid" || typeof e.detail !== "string") return null;
  const head = HEAD.exec(e.detail);
  if (!head) return null;
  const raw = head[1] ?? "";
  const msg = head[2] ?? "";
  for (const [re, what] of NAMED) {
    const m = re.exec(msg);
    if (m) return { raw, what, word: m[1] ?? "" };
  }
  const m = ENUM.exec(msg);
  if (m) return { raw, what: "value", word: m[1] ?? "", options: (m[2] ?? "").split(" ") };
  return null;
}

// distance is the edit distance (insert, delete, change one letter) of two words.
function distance(a: string, b: string): number {
  let prev = Array.from({ length: b.length + 1 }, (_, j) => j);
  for (let i = 1; i <= a.length; i++) {
    const row = [i];
    for (let j = 1; j <= b.length; j++) {
      const same = a[i - 1] === b[j - 1] ? 0 : 1;
      row.push(Math.min((prev[j] ?? 0) + 1, (row[j - 1] ?? 0) + 1, (prev[j - 1] ?? 0) + same));
    }
    prev = row;
  }
  return prev[b.length] ?? 0;
}

// closest is the name within two letters of the word, the nearest, the first on a tie. A word of
// one or two letters is left alone when the fix would rewrite all of it. With none near, a word of
// three letters or more that is part of a name is that name.
function closest(word: string, names: string[]): string | null {
  const w = word.toLowerCase();
  let best: string | null = null;
  let bestDistance = Infinity;
  for (const name of names) {
    const d = distance(w, name.toLowerCase());
    if (d <= MAX_DISTANCE && d < w.length && d < bestDistance) {
      best = name;
      bestDistance = d;
    }
  }
  if (best !== null || w.length < MIN_CONTAINED) return best;
  // A word that is part of a name ("progres" of "in_progress") is the name, when no name is near.
  return names.find((name) => name.toLowerCase().includes(w)) ?? null;
}

function uniq(names: string[]): string[] {
  return [...new Set(names)];
}

function namesFor(p: Problem, schemas: SchemaView[]): string[] {
  switch (p.what) {
    case "status":
      return uniq([
        ...schemas.flatMap((s) => (s.statuses ?? []).map((x) => x.name)),
        ...CATEGORIES,
      ]);
    case "kind":
      return KINDS.map((k) => k.name);
    case "key":
      return uniq([...QUERY_KEYS, ...schemas.flatMap((s) => Object.keys(s.props ?? {}))]);
    case "value":
      return p.options ?? [];
  }
}

// The term with the wrong name changed for the right one.
function fixed(p: Problem, name: string): string {
  if (p.what === "key") {
    const m = /^(-?)([^:<>=]*)(.*)$/s.exec(p.raw);
    return m && m[2] === p.word ? `${m[1]}${name}${m[3]}` : p.raw;
  }
  // The value is after the operator; the key may hold the same letters.
  const at = p.raw.search(/[:<>=]/);
  const rest = p.raw.slice(at + 1);
  // Only the value that is wrong changes, not a part of another value of the term ("todo,tod").
  const values = splitValues(rest).map((v) => (v === p.word ? name : v));
  return `${p.raw.slice(0, at + 1)}${values.join(",")}`;
}

// splitValues cuts a value at the commas outside double quotes, as the query parser does.
function splitValues(rest: string): string[] {
  const out: string[] = [];
  let cur = "";
  let quoted = false;
  for (const ch of rest) {
    if (ch === '"') quoted = !quoted;
    if (ch === "," && !quoted) {
      out.push(cur);
      cur = "";
    } else {
      cur += ch;
    }
  }
  out.push(cur);
  return out;
}

/**
 * The term the error is about, written with the name it is closest to (within two letters):
 * "status:in_progres" is "status:in_progress". Null when the error is not one of a name, or no
 * name is near.
 */
export function suggestTerm(error: unknown, schemas: SchemaView[]): string | null {
  const p = problemOf(error);
  if (!p) return null;
  const name = closest(p.word, namesFor(p, schemas));
  if (name === null) return null;
  const to = fixed(p, name);
  return to === p.raw ? null : to;
}

/**
 * The query with the term the error is about changed for `to` (what suggestTerm gave). Null when
 * the term is not in q: it is in the part the screen fixes, which the user cannot change.
 */
export function replaceTerm(q: string, error: unknown, to: string): string | null {
  const p = problemOf(error);
  if (!p) return null;
  const terms = splitTerms(q);
  const at = terms.indexOf(p.raw);
  if (at < 0) return null;
  terms[at] = to;
  return terms.join(" ");
}
