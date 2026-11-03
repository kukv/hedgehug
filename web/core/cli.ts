// The hh command that does what a dialog or a filter does, shown under it
// for the user to copy (Foundations: the same operation as a command).
import type { ViewSaveBody } from "./views";

const PLAIN = /^[\p{L}\p{N}_\-.,:/@%+=]+$/u;

// quoteArg writes s as one argument of a shell. Only an argument with a
// space or a symbol is put in double quotes; inside them " \ $ and ` are
// escaped with a backslash.
export function quoteArg(s: string): string {
  // ! is expanded by an interactive bash or zsh even inside double quotes,
  // and a leading = by zsh: single quotes keep both as they are.
  if (s.includes("!") || s.startsWith("=")) return `'${s.replaceAll("'", "'\\''")}'`;
  if (s !== "" && PLAIN.test(s)) return s;
  return `"${s.replace(/["\\$`]/g, "\\$&")}"`;
}

// A stand-in for an argument that is not known yet, shown as <名前>. It is not quoted: a shell
// reads <名前> as a redirection from a file and refuses the line, so a command copied before the
// input is complete makes nothing.
export class Placeholder {
  readonly text: string;
  constructor(text: string) {
    this.text = `<${text}>`;
  }
}
type Arg = string | Placeholder;

// withPositionals writes a command whose arguments the user typed (a title,
// a query, a name). When one starts with - it would be read as a flag, so the
// flags go first and -- comes before it. Otherwise the order is the one the
// command has always had: positionalsFirst puts them before the flags.
function withPositionals(
  head: string[],
  pos: Arg[],
  flags: Arg[],
  positionalsFirst: boolean,
): string {
  if (pos.some((p) => typeof p === "string" && p.startsWith("-")))
    return cliLine([...head, ...flags, "--", ...pos]);
  return cliLine(positionalsFirst ? [...head, ...pos, ...flags] : [...head, ...flags, ...pos]);
}

// cliLine is the command for args, without the "$ " that the screen puts in
// front.
export function cliLine(args: Arg[]): string {
  return ["hh", ...args.map((a) => (a instanceof Placeholder ? a.text : quoteArg(a)))].join(" ");
}

export function cliAdd(i: {
  title: Arg;
  parent?: string;
  after?: string;
  before?: string;
  workspace?: string;
  store?: string;
  kind?: string;
  body?: string;
  props?: Record<string, string>;
}): string {
  // The GUI names where the task goes: hh must not link it to the repository it runs in.
  const a: string[] = ["--no-repo"];
  if (i.store) a.push("--store", i.store);
  if (i.workspace) a.push("--workspace", i.workspace);
  if (i.parent) a.push("--parent", i.parent);
  if (i.after) a.push("--after", i.after);
  if (i.before) a.push("--before", i.before);
  if (i.kind) a.push("--kind", i.kind);
  if (i.body) a.push("--body", i.body);
  for (const [k, v] of Object.entries(i.props ?? {})) a.push("-p", `${k}=${v}`);
  return withPositionals(["add"], [i.title], a, true);
}

export function cliLs(q: string, tree: boolean, store?: string): string {
  const a: string[] = [];
  if (store) a.push("--store", store);
  if (tree) a.push("--tree");
  return withPositionals(["ls"], q.trim() !== "" ? [q] : [], a, false);
}

// cliSet is hh set; a key that ends with + or - adds to or removes from a
// list (labels+ is labels+=x), an empty value clears.
export function cliSet(id: string, assigns: Record<string, string>): string {
  return cliLine(["set", id, ...Object.entries(assigns).map(([k, v]) => `${k}=${v}`)]);
}

export function cliMv(
  id: string,
  to: { parent?: string; root?: boolean; before?: string; after?: string; workspace?: Arg },
): string {
  const a: Arg[] = ["mv", id];
  if (to.parent) a.push("--parent", to.parent);
  if (to.workspace) a.push("--workspace", to.workspace);
  if (to.root) a.push("--root");
  if (to.before) a.push("--before", to.before);
  if (to.after) a.push("--after", to.after);
  return cliLine(a);
}

export const cliRm = (id: string): string => cliLine(["rm", id]);
export const cliRestore = (id: string): string => cliLine(["restore", id]);
export const cliRestoreAt = (id: string, commit: string): string =>
  cliLine(["restore", id, "--at", commit]);

// cliComment is hh comment add <id>, hh comment edit <comment> or hh comment
// rm <comment>; edit and rm take the ULID of the comment as cid.
export function cliComment(id: string, kind: "add" | "edit" | "rm", cid?: string): string {
  if (kind === "add") return cliLine(["comment", "add", id]);
  return cliLine(["comment", kind, cid ?? id]);
}

export const cliWatch = (id: string, on: boolean): string =>
  cliLine([on ? "watch" : "unwatch", id]);

// cliInbox is hh inbox; all also lists the items read.
export function cliInbox(i: { all?: boolean; store?: string } = {}): string {
  const a: string[] = ["inbox"];
  if (i.all) a.push("--all");
  if (i.store) a.push("--store", i.store);
  return cliLine(a);
}

// cliLog is hh log; without --store it reads every store, as the screen does.
export function cliLog(workspaces: string[]): string {
  return cliLine(workspaces.length > 0 ? ["log", "--workspace", workspaces.join(",")] : ["log"]);
}

export function cliSync(store?: string): string {
  return cliLine(store ? ["sync", "--store", store] : ["sync"]);
}

// cliWorkspaceAdd is hh workspace add; --store is the global flag that picks the store.
export function cliWorkspaceAdd(i: { name: Arg; prefix: Arg; store: string }): string {
  return withPositionals(
    ["workspace", "add"],
    [i.name],
    ["--prefix", i.prefix, "--store", i.store],
    true,
  );
}

// cliWorkspaceRename is hh workspace rename <workspace> <name>.
export function cliWorkspaceRename(i: { workspace: string; name: Arg; store: string }): string {
  return withPositionals(
    ["workspace", "rename"],
    [i.workspace, i.name],
    ["--store", i.store],
    true,
  );
}

// cliWorkspaceSetPrefix is hh workspace set-prefix <workspace> <P>.
export function cliWorkspaceSetPrefix(i: {
  workspace: string;
  prefix: Arg;
  store: string;
}): string {
  return withPositionals(
    ["workspace", "set-prefix"],
    [i.workspace, i.prefix],
    ["--store", i.store],
    true,
  );
}

export const cliWorkspaceRm = (i: { workspace: string; store: string }): string =>
  cliLine(["workspace", "rm", i.workspace, "--store", i.store]);

// cliViewSave is hh view save <name>; a flag is written for each setting body holds (the others stay
// as saved, as a PUT leaves them). --store is the global flag that picks the store.
export function cliViewSave(i: { name: Arg; store?: string; body: ViewSaveBody }): string {
  const b = i.body;
  const a: Arg[] = [];
  if (b.layout !== undefined) a.push("--layout", b.layout);
  if (b.query !== undefined) a.push("--query", b.query);
  if (b.columns !== undefined) a.push("--columns", b.columns.join(","));
  if (b.group_by !== undefined) a.push("--group-by", b.group_by);
  if (b.kanban_columns !== undefined) a.push("--kanban-columns", b.kanban_columns);
  if (b.swimlane !== undefined) a.push("--swimlane", b.swimlane);
  if (i.store) a.push("--store", i.store);
  return withPositionals(["view", "save"], [i.name], a, true);
}

export function cliViewShow(name: string, store?: string): string {
  return withPositionals(["view", "show"], [name], store ? ["--store", store] : [], true);
}

export function cliViewRm(name: string, store?: string): string {
  return withPositionals(["view", "rm"], [name], store ? ["--store", store] : [], true);
}

export function cliViewRename(name: string, to: Arg, store?: string): string {
  return withPositionals(["view", "rename"], [name, to], store ? ["--store", store] : [], true);
}

export function cliStoreInit(i: {
  name: Arg;
  prefix: Arg;
  workspace?: string;
  remotes?: string[];
}): string {
  const a: Arg[] = ["--prefix", i.prefix];
  if (i.workspace) a.push("--workspace", i.workspace);
  for (const r of i.remotes ?? []) a.push("--remote", r);
  return withPositionals(["store", "init"], [i.name], a, true);
}

export function cliScopeShow(id: string, store?: string): string {
  return cliLine(["schema", "scope", "show", id, ...(store ? ["--store", store] : [])]);
}

export function cliScopeRm(id: string, store?: string): string {
  return cliLine(["schema", "scope", "rm", id, ...(store ? ["--store", store] : [])]);
}

// cliScopeSet is hh schema scope set with the parts that are given (a part that is left out is not
// touched). A part that is empty takes the override off: "" is its value on the command line.
// With no part at all the line is not complete yet: <変更> stands where the parts go.
export function cliScopeSet(i: {
  id: string;
  statuses?: string[];
  transitions?: Record<string, string[]>;
  required?: Record<string, string[]>;
  store?: string;
}): string {
  const lists = (flag: string, m: Record<string, string[]> | undefined): string[] => {
    if (!m) return [];
    const entries = Object.entries(m);
    if (entries.length === 0) return [flag, ""];
    return entries.flatMap(([k, v]) => [flag, `${k}=${v.join(",")}`]);
  };
  const a: Arg[] = ["schema", "scope", "set", i.id];
  if (i.statuses) a.push("--statuses", i.statuses.join(","));
  a.push(...lists("--transition", i.transitions), ...lists("--required", i.required));
  if (a.length === 4) a.push(new Placeholder("変更"));
  if (i.store) a.push("--store", i.store);
  return cliLine(a);
}
