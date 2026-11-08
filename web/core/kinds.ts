// The six kinds of task. They are fixed: the table here is the same as the one in
// internal/app/domain/schema/kinds.go, and schema.yaml has no say in it.

export type KindRole = "group" | "item";

export type Kind = {
  name: string;
  label: string;
  role: KindRole;
  // The kinds it may sit under; null stands for the top.
  parents: (string | null)[];
};

export const KINDS: readonly Kind[] = [
  { name: "project", label: "Project", role: "group", parents: [null] },
  { name: "epic", label: "Epic", role: "group", parents: ["project"] },
  { name: "story", label: "Story", role: "item", parents: ["project", "epic"] },
  { name: "task", label: "Task", role: "item", parents: ["project", "epic"] },
  { name: "bug", label: "Bug", role: "item", parents: ["project", "epic"] },
  { name: "step", label: "Step", role: "item", parents: ["story", "task", "bug"] },
];

const kindOf = (name: string): Kind | undefined => KINDS.find((k) => k.name === name);

export const kindLabel = (name: string): string => kindOf(name)?.label ?? name;

// canPlace is whether a task of kind may sit under a parent of parentKind (null: at the top).
export const canPlace = (kind: string, parentKind: string | null): boolean =>
  kindOf(kind)?.parents.includes(parentKind) ?? false;

// isGroup is whether kind groups work: a project or an epic.
export const isGroup = (kind: string): boolean => kindOf(kind)?.role === "group";

// countsForProgress is whether a task of kind counts in the progress of its project or
// epic: a story, a task or a bug. A step does not.
export const countsForProgress = (kind: string): boolean =>
  kind === "story" || kind === "task" || kind === "bug";

// defaultKindUnder is the kind of a task made without one, from the kind of its parent.
export function defaultKindUnder(parentKind: string | null): string {
  if (parentKind === null) return "project";
  if (parentKind === "story" || parentKind === "task" || parentKind === "bug") return "step";
  return "task";
}

// kindsUnder are the kinds that may sit under a parent of parentKind, in the order of KINDS.
export const kindsUnder = (parentKind: string | null): Kind[] =>
  KINDS.filter((k) => k.parents.includes(parentKind));

// parentRule says where a kind can be put, for the tooltip of a kind that cannot be
// chosen: 「Epic のどの下に置けるかは Project だけです」.
export function parentRule(kind: string): string {
  const k = kindOf(kind);
  if (!k || k.parents.length === 0) return `${kindLabel(kind)} は親に置けません`;
  if (k.parents.length === 1 && k.parents[0] === null) return `${k.label} は最上位にだけ置けます`;
  const names = k.parents.map((p) => (p === null ? "最上位" : kindLabel(p)));
  return `${k.label} のどの下に置けるかは ${names.join("・")} だけです`;
}

// kindChoices are the kinds to pick from for a task of kind under a parent of parentKind (null: no
// parent; undefined: a parent whose kind is not known, so none is ruled out here and the server
// decides). One that cannot be chosen says why. The kind the task has stays chosen.
export const kindChoices = (
  kind: string,
  parentKind: string | null | undefined,
): { value: string; label: string; disabled: boolean; title?: string }[] =>
  KINDS.map((k) => {
    const usable = k.name === kind || parentKind === undefined || canPlace(k.name, parentKind);
    return {
      value: k.name,
      label: k.label,
      disabled: !usable,
      title: usable ? undefined : parentRule(k.name),
    };
  });
