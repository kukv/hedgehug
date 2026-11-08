import type { MigrateTarget } from "./MigrateDialog.svelte";

/** What a row or a button of a schema tab asks the screen to open. */
export type SchemaAction =
  | { kind: "migrate"; target: MigrateTarget; old: string; names: string[] }
  | { kind: "add"; target: "status" | "kind" | "prop" | "relation" }
  | { kind: "edit"; target: "status" | "kind" | "prop" | "relation"; name: string }
  | { kind: "delete"; target: "status" | "kind" | "prop"; name: string };
