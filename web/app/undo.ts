import { type Schemas, unwrap } from "../core/api/client";
import { errorMessage } from "../core/messages";
import type { AppCore } from "./context";
import { opLabel } from "./detail/history-labels";
import type { Route } from "./router";
import { toast } from "./toast.svelte";

type UndoWrite = Schemas["UndoWrite"];

// undoMessage says what POST /api/undo did: the operation taken back, and what it kept because
// others changed it after. commit is null when every field it changed had been changed since.
export function undoMessage(r: UndoWrite): string {
  const what = `${r.undone.id ? `${r.undone.id} の` : ""}${opLabel(r.undone.op)}`;
  const kept = r.kept?.length ?? 0;
  if (r.commit === null) {
    return `${what}は、変えた値がすべてあとで別の人に変えられていたため、何も戻しませんでした`;
  }
  return kept > 0
    ? `${what}を取り消しました · あとで別の人が変えた ${kept} 件の値は残しました`
    : `${what}を取り消しました`;
}

// routeStore is the store the screen is about: the store of a workspace, of the settings page, or
// of the project or task it shows (the cache may not have it yet). Home shows the overview of the
// first workspace, so it is that one's store. Elsewhere there is none.
export function routeStore(
  route: Route,
  taskOf: (ulid: string) => { store: string } | undefined,
  first?: { store: string },
): string | undefined {
  if (route.name === "workspace") return route.store;
  if (route.name === "settings") return route.store || undefined;
  if (route.name === "home") return first?.store;
  if (route.name === "project" || route.name === "task") return taskOf(route.ulid)?.store;
  return undefined;
}

// runUndo takes back the last operation of the user (POST /api/undo, as hh undo) and says how it
// went. It does not ask first: the spec holds that an undo is not hard to take back. The store is
// the one the screen is about; without it the server picks one as the CLI does, which asks the
// user to choose when there are several.
export async function runUndo(c: Pick<AppCore, "api" | "cache">, store?: string): Promise<void> {
  try {
    const r = await unwrap(c.api.POST("/api/undo", { params: { query: { store } } }));
    // The commit is this tab's own: its tasks.changed is not shown as a change from outside.
    c.cache.noteOwnCommit(r.commit);
    toast({ message: undoMessage(r) });
  } catch (e) {
    toast({ message: errorMessage(e) });
  }
}
