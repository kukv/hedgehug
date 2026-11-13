import { Refused } from "../../model/errors";
import type { Direction, WorkspacesPort } from "../../port/workspaces";
import type { Catalog, StoreNode } from "../catalog/catalog";
import { readOnlyNote } from "../catalog/readonly";
import type { Toasts } from "../toasts/toasts";

// サイドバーのワークスペースのメニューの「上へ移動」「下へ移動」(04「並び順と並べ替え」)。

// canMoveWorkspace は、動かせるか。先頭は上へ、末尾は下へ動かせず、閲覧のみの Store では動かせない。
export function canMoveWorkspace(store: StoreNode, workspaceId: string, direction: Direction) {
  if (store.readOnly) return false;
  const index = store.workspaces.findIndex((w) => w.id === workspaceId);
  if (index < 0) return false;
  return direction === "up" ? index > 0 : index < store.workspaces.length - 1;
}

// deleteMenuItem は、メニュー末尾の「削除…」(B5b-11)。閲覧のみの Store では無効にして、その理由を添える。
export function deleteMenuItem(store: StoreNode) {
  return {
    id: "delete",
    label: "削除…",
    disabled: store.readOnly,
    hint: store.readOnly ? readOnlyNote : undefined,
  };
}

export interface WorkspaceMoverDeps {
  catalog: Catalog;
  port: WorkspacesPort;
  toasts: Toasts;
}

export interface WorkspaceMover {
  // move は、1 つ動かして一覧を読み直す。断りはエラーのトーストにする。成功のトーストは出さない。
  move(storeId: string, workspaceId: string, direction: Direction): Promise<void>;
}

export function createWorkspaceMover(deps: WorkspaceMoverDeps): WorkspaceMover {
  return {
    async move(storeId, workspaceId, direction) {
      try {
        await deps.port.move(storeId, workspaceId, direction);
      } catch (e) {
        if (!(e instanceof Refused)) throw e;
        deps.toasts.show(e.message, { error: true });
      }
      await deps.catalog.load();
    },
  };
}
