import type { MigrateTarget } from "./MigrateDialog.svelte";

/** What a row or a button of a schema tab asks the screen to open. */
export type SchemaAction =
  | { kind: "migrate"; target: MigrateTarget; old: string; names: string[] }
  | { kind: "add"; target: "status" | "prop" }
  | { kind: "edit"; target: "status" | "prop"; name: string }
  | { kind: "delete"; target: "status" | "prop"; name: string };
