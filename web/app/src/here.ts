import { type Catalog, get, type Here, type Screen } from "@hedgehug/core";

// hereOf は、作成ダイアログの初期値に使う、いま開いている画面のワークスペースとプロジェクト(02)。
// wire.ts が 300 行の上限に近いので、ここに分けた。
export function hereOf(screen: Screen, catalog: Catalog): Here {
  const c = get(screen.current);
  if (c?.name === "workspace") return { workspaceId: c.workspaceId };
  if (c?.name === "project") {
    return {
      workspaceId: catalog.project(c.storeId, c.projectId)?.workspaceId,
      projectId: c.projectId,
    };
  }
  return {};
}
