import {
  createProjectMover,
  createRenameProjectDialog,
  menuAt,
  type ProjectMenuId,
  type ProjectMenuItem,
  type ProjectNode,
  projectMenu,
  readonly,
  type StoreNode,
  type WorkspaceNode,
  writable,
} from "@hedgehug/core";
import { projectEditor, projectsPort } from "./projectEditor";
import { catalog, itemDialog, navigation, toasts } from "./wire";
import { projectDelete } from "./wireActions";

// ツリーのプロジェクトの行の操作(E3a-12〜14)の組み立て。wire.ts が 300 行の上限なので、ここに置く。
// 検索の画面とワークスペースの画面のツリーが使う。

// 並べ替え(サイドバーの並びを 1 つ動かし、catalog を読み直す)。
export const projectMover = createProjectMover({ catalog, port: projectsPort, toasts });

// 「名前を変更…」のダイアログ。検査と断りの文は、概要のタイトルの変更(projectEditor)と同じ。
export const renameProject = createRenameProjectDialog({
  rename: (project, title) => projectEditor.renameOrMessage(project, title),
});

interface OpenMenu {
  x: number;
  y: number;
  items: ProjectMenuItem[];
  store: StoreNode;
  project: ProjectNode;
}

const menu = writable<OpenMenu | null>(null);
export const projectMenuState = readonly(menu);

// openProjectMenu は、プロジェクトの行のメニューを開く。rect は「⋯」の getBoundingClientRect()。
export function openProjectMenu(
  store: StoreNode,
  workspace: WorkspaceNode,
  project: ProjectNode,
  rect: { left: number; bottom: number },
) {
  const { x, y } = menuAt(rect);
  menu.set({ x, y, items: projectMenu(store, workspace, project.id), store, project });
}

export function closeProjectMenu() {
  menu.set(null);
}

let current: OpenMenu | null = null;
menu.subscribe((m) => {
  current = m;
});

// selectProjectMenu は、メニューの項目を実行して閉じる。
export function selectProjectMenu(id: ProjectMenuId) {
  const m = current;
  closeProjectMenu();
  if (!m) return;
  const { store, project } = m;
  if (id === "open") {
    navigation.go({
      name: "project",
      store: store.name,
      project: project.displayId,
      tab: "overview",
    });
  } else if (id === "add-epic") {
    void itemDialog.open({
      workspaceId: project.workspaceId,
      projectId: project.id,
      kind: "Epic",
      parentId: null,
    });
  } else if (id === "rename") {
    renameProject.open(project);
  } else if (id === "delete") {
    void projectDelete.open(project);
  } else {
    void projectMover.move(store.id, project.id, id);
  }
}

// 画面を移ったら、出しているメニューとダイアログを閉じる(01「画面の移り方」)。
navigation.route.subscribe(() => {
  closeProjectMenu();
  renameProject.close();
});
