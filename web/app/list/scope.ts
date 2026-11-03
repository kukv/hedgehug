// What a screen of tasks lists: one workspace, one project, or every store (the saved views, the
// query screen and the tasks assigned to me).
export type ListScope =
  | { kind: "workspace"; store: string; ulid: string }
  | { kind: "project"; ulid: string }
  | { kind: "all" }; // views, the query screen, my tasks: every store
