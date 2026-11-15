import { readOnlyBand } from "../catalog/readonly";
import type { SearchRow } from "./types";

// 検索の画面に出す文(04「検索」「空のとき」、E1-8・E1-18)。

export interface EmptyText {
  title: string;
  body: string | null;
  // create は、「作成」のボタンを出すか。
  create: boolean;
}

// emptyText は、本体が 0 件のときの文。行があれば null。書き方に誤りがあるときは、本体を 0 件の表示にする。
export function emptyText(query: string, rows: number, errors: string[]): EmptyText | null {
  if (errors.length === 0 && rows > 0) return null;
  if (errors.length > 0 || query.trim() !== "") {
    return {
      title: "一致するものはありません",
      body: "絞り込みを変えてみてください。",
      create: false,
    };
  }
  return { title: "まだ何もありません", body: null, create: true };
}

// treeEmptyText は、ツリーの本体が空のときの文(E3a-19)。tree は描いた木(rows は描いた行の数。プロジェクトの行も数える)。
// 表示中の Store にプロジェクトがなければ「プロジェクトはまだありません」。行が 0 件か、書き方に誤りがあれば
// 「一致するものはありません」。検索の画面には「作成」のボタンを置かない。
export function treeEmptyText(
  tree: { hasProjects: boolean; rows: number },
  errors: string[],
): EmptyText | null {
  if (!tree.hasProjects) {
    return {
      title: "プロジェクトはまだありません",
      body: "ワークスペースにプロジェクトを作ると、ここに Epic・Story・Task・Bug・Step の木が並びます。",
      create: false,
    };
  }
  if (errors.length === 0 && tree.rows > 0) return null;
  return {
    title: "一致するものはありません",
    body: "絞り込みを変えてみてください。",
    create: false,
  };
}

// readOnlyBands は、閲覧のみの Store のアイテムが含まれるとき、その Store ごとに 1 本の帯の文(最初に出た順)。
export function readOnlyBands(rows: SearchRow[]): string[] {
  const seen = new Map<string, string>();
  for (const r of rows) {
    if (r.readOnly && !seen.has(r.storeId)) seen.set(r.storeId, readOnlyBand(r.storeName));
  }
  return [...seen.values()];
}

// deletedNote は、削除済みの行に添える文(08「削除済み」)。daysLeft が 0 以下なら保持日数を過ぎている
// (「あと 0 日」とは出さない)。
export function deletedNote(daysLeft: number): string {
  return daysLeft <= 0 ? "保持日数を過ぎています" : `あと ${daysLeft} 日で完全に削除`;
}

// assigneeText は、担当者の表示名。メンバーにいなければ「不明なユーザー」(01)、担当者がなければ null。
export function assigneeText(a: SearchRow["assignee"]): string | null {
  if (!a) return null;
  return a.name ?? "不明なユーザー";
}
