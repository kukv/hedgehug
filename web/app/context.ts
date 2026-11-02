import { getContext, setContext } from "svelte";
import type { Api } from "../core/api/client";
import type { TaskCache } from "../core/cache";
import type { Connection } from "../core/connection";
import type { SaveController } from "../core/save";
import type { SchemaCache } from "../core/schema";
import type { StoreDirectory } from "../core/stores";

export type AppCore = {
  api: Api;
  conn: Connection;
  cache: TaskCache;
  save: SaveController;
  stores: StoreDirectory;
  schema: SchemaCache;
};

/** The key of the context; tests pass it to render to replace the core. */
export const CORE_KEY = Symbol("hedgehug.core");

export function setCore(c: AppCore): void {
  setContext(CORE_KEY, c);
}

export function core(): AppCore {
  const c = getContext<AppCore | undefined>(CORE_KEY);
  if (!c) throw new Error("core() was called outside the App");
  return c;
}
