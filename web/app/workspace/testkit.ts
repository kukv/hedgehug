import type { WorkspaceEntry } from "../../core/stores";

/** A workspace of the directory, for the tests of the workspace screens. */
export function entry(over: Partial<WorkspaceEntry> = {}): WorkspaceEntry {
  return {
    store: "shared",
    tone: "purple",
    ulid: "W1",
    name: "release",
    prefix: "REL",
    projects: [],
    tasks: 24,
    deleted: 1,
    prefixSet: "2026-10-29T12:00:00Z",
    ...over,
  };
}

/** What GET /api/stores answers for one store. */
export function storeItem(name: string, isDefault = false) {
  return { name, default: isDefault, path: `/data/${name}`, remotes: [] };
}

/** The answer of a write to a workspace (WorkspaceWrite). */
export function written(
  w: Partial<{ ulid: string; name: string; id_prefix: string }> = {},
  extra: { commit?: string | null; ids?: { old: string; new: string }[] } = {},
) {
  return {
    workspace: {
      ulid: "W1",
      name: "release",
      id_prefix: "REL",
      prefix_set: "2026-11-03T00:00:00Z",
      tasks: 0,
      deleted: 0,
      ...w,
    },
    commit: "c1",
    ...extra,
  };
}
