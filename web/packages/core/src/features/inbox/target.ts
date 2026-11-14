import type { InboxRow, InboxTargetKind } from "../../model/inbox";
import type { Route } from "../router/route";

// goneTextOf は、削除済みの対象の表示(リンクにしない)。削除済みでなければ null。
export function goneTextOf(kind: InboxTargetKind): string | null {
  switch (kind) {
    case "deleted_item":
      return "削除されたアイテム";
    case "deleted_workspace":
      return "削除されたワークスペース";
    case "deleted_project":
      return "削除されたプロジェクト";
    default:
      return null;
  }
}

// projectRouteOf は、プロジェクトを指す行の行き先(プロジェクトの概要)。moved_project は旧 ID のアドレスを開き、
// プロジェクトの画面がいまの場所を探す(C4b1-12)。プロジェクトでない行と削除済みは null。
export function projectRouteOf(row: InboxRow): Route | null {
  const t = row.target;
  if (t.kind !== "project" && t.kind !== "moved_project") return null;
  return { name: "project", store: row.store.name, project: t.display, tab: "overview" };
}
