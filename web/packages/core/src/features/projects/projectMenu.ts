import { Refused } from "../../model/errors";
import type { ProjectsPort } from "../../port/projects";
import type { Direction } from "../../port/workspaces";
import type { Catalog, StoreNode, WorkspaceNode } from "../catalog/catalog";
import { readOnlyNote } from "../catalog/readonly";
import type { Toasts } from "../toasts/toasts";

// ツリーのプロジェクトの行のメニュー(04「ツリー」の「行」、E3a-13)。

export type ProjectMenuId = "open" | "add-epic" | "up" | "down" | "rename" | "delete";

export interface ProjectMenuItem {
  id: ProjectMenuId;
  label: string;
  disabled: boolean;
  hint?: string;
}

// canMoveProject は、サイドバーの並び(そのワークスペースの削除されていないすべてのプロジェクト)の中で動かせるか。
// 先頭は上へ、末尾は下へ動かせない。
export function canMoveProject(workspace: WorkspaceNode, projectId: string, dir: Direction) {
  const index = workspace.projects.findIndex((p) => p.id === projectId);
  if (index < 0) return false;
  return dir === "up" ? index > 0 : index < workspace.projects.length - 1;
}

// projectMenu は、メニューの項目。閲覧のみの Store では「開く」以外を無効にして理由を添える。先頭・末尾の無効には、
// 理由の文を付けない(サイドバーのワークスペースのメニューと同じ)。
export function projectMenu(
  store: StoreNode,
  workspace: WorkspaceNode,
  projectId: string,
): ProjectMenuItem[] {
  const write = (id: ProjectMenuId, label: string, movable = true): ProjectMenuItem => ({
    id,
    label,
    disabled: store.readOnly || !movable,
    hint: store.readOnly ? readOnlyNote : undefined,
  });
  return [
    { id: "open", label: "開く", disabled: false },
    write("add-epic", "Epic を追加"),
    write("up", "上へ移動", canMoveProject(workspace, projectId, "up")),
    write("down", "下へ移動", canMoveProject(workspace, projectId, "down")),
    write("rename", "名前を変更…"),
    write("delete", "削除…"),
  ];
}

export interface ProjectMoverDeps {
  catalog: Catalog;
  port: Pick<ProjectsPort, "move">;
  toasts: Toasts;
}

export interface ProjectMover {
  // move は、1 つ動かして一覧を読み直す。断りはエラーのトーストにする。成功のトーストは出さない。
  move(storeId: string, projectId: string, dir: Direction): Promise<void>;
}

export function createProjectMover(deps: ProjectMoverDeps): ProjectMover {
  return {
    async move(storeId, projectId, dir) {
      try {
        await deps.port.move(storeId, projectId, dir);
      } catch (e) {
        if (!(e instanceof Refused)) throw e;
        deps.toasts.show(e.message, { error: true });
      }
      await deps.catalog.load();
    },
  };
}
