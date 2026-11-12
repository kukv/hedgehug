import type { ProjectStatus } from "../../model/project";

// ProjectCategory は、プロジェクトのステータスの点の色の分類(04「Store ごとのブロック」)。
export type ProjectCategory = "todo" | "doing" | "done" | "canceled";

// projectCategory は、Todo と Pending を未着手、InProgress を進行中、Done を完了、Cancel を中止にする。
export function projectCategory(status: ProjectStatus): ProjectCategory {
  switch (status) {
    case "InProgress":
      return "doing";
    case "Done":
      return "done";
    case "Cancel":
      return "canceled";
    default:
      return "todo";
  }
}

// projectStatuses は、作成ダイアログのステータスの選択肢の並び(02「プロジェクト」)。
export const projectStatuses: ProjectStatus[] = ["Todo", "InProgress", "Done", "Pending", "Cancel"];
