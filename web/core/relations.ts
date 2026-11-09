// The five relations between tasks. They are fixed: the table here is the same as the one in
// internal/app/domain/schema/relations.go, and schema.yaml has no say in it. Each is a built-in
// field of a task (a list of tasks); the task that holds it shows label, the one it points at
// shows inverseLabel under the inverse name.

export type Relation = {
  name: string;
  label: string;
  inverse: string;
  inverseLabel: string;
};

// The names of the relation fields of a task (Relation.name).
export type RelationField = "blocked_by" | "causes" | "relates" | "duplicates" | "clones";

export const RELATIONS: readonly Relation[] = [
  { name: "blocked_by", label: "Blocked by", inverse: "blocks", inverseLabel: "Blocks" },
  { name: "causes", label: "Causes", inverse: "caused_by", inverseLabel: "Caused by" },
  { name: "relates", label: "Relates to", inverse: "related_by", inverseLabel: "Related by" },
  {
    name: "duplicates",
    label: "Duplicates",
    inverse: "duplicated_by",
    inverseLabel: "Duplicated by",
  },
  { name: "clones", label: "Clones", inverse: "cloned_by", inverseLabel: "Cloned by" },
];

// relationOf finds a relation by its name (inverse is false) or by the name of its other
// direction (inverse is true).
export function relationOf(name: string): { name: string; inverse: boolean } | undefined {
  for (const r of RELATIONS) {
    if (name === r.name) return { name: r.name, inverse: false };
    if (name === r.inverse) return { name: r.name, inverse: true };
  }
  return undefined;
}

// relationLabel is the display name of a relation, or of its other direction; a name that is
// neither comes back as it is.
export function relationLabel(name: string): string {
  for (const r of RELATIONS) {
    if (name === r.name) return r.label;
    if (name === r.inverse) return r.inverseLabel;
  }
  return name;
}

export const inverseRelationNames = (): string[] => RELATIONS.map((r) => r.inverse);
