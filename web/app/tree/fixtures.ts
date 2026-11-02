import type { TaskView } from "../../core/api/client";
import type { SchemaView } from "../../core/schema";

export const schema = {
  statuses: [
    { name: "backlog", category: "todo", default: true },
    { name: "todo", category: "todo" },
    { name: "doing", category: "doing", default: true },
    { name: "done", category: "done", default: true },
    { name: "canceled", category: "canceled", default: true },
  ],
  kinds: [
    { name: "project", label: "プロジェクト", role: "container", parents: ["root"] },
    { name: "epic", label: "エピック", role: "container", parents: ["root", "project"] },
    { name: "story", label: "ストーリー", role: "work", parents: ["root", "project", "epic"] },
    {
      name: "task",
      label: "タスク",
      role: "work",
      parents: ["root", "project", "epic", "story", "task"],
      default: true,
    },
    { name: "bug", label: "バグ", role: "work", parents: ["root", "project", "epic", "story"] },
  ],
  props: {},
  scoped: [],
  workflow: { transitions: {}, required: {} },
} as unknown as SchemaView;

export function task(
  ulid: string,
  kind: string,
  parent: string | null,
  extra: Partial<TaskView> = {},
): TaskView {
  return {
    ulid,
    id: `REL-${ulid.replace(/\D/g, "") || "0"}`,
    store: "shared",
    workspace: { ulid: "W1", name: "release", id_prefix: "REL" },
    kind,
    title: `title ${ulid}`,
    parent,
    rank: "a",
    status: "todo",
    assignees: [],
    labels: [],
    blocked_by: [],
    props: {},
    created: "2026-11-01T00:00:00Z",
    updated: "2026-11-01T00:00:00Z",
    deleted_at: null,
    problems: [],
    ...extra,
  } as TaskView;
}
