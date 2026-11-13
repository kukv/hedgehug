import type {
  LinkChecks,
  LinkKind,
  LinkReview,
  LinkRow,
  LinkState,
  LinkTarget,
} from "../../model/link";

// GitHub の欄の文言(07「PR と Issue のリンク」「GitHub を使わない場合」、02「GitHub」)。
// unlinkFailedMessage は、解除が断りでも「ない」でもない失敗(通信の失敗など)のときのエラーのトースト(07「解除」)。

export const noLinksText = "リンクはありません";
export const unlinkedMessage = "リンクを解除しました";
export const unlinkFailedMessage = "リンクを解除できませんでした";
export const noAccountNote = "GitHub のアカウントを登録すると使えます";
export const markerCopiedMessage = "コピーしました。PR の説明に貼ってください";
export const markerCopyFailedMessage = "コピーできませんでした。";

export const markerOf = (itemId: string) => `hedgehug:${itemId}`;

export const linkDialogTitle = (k: LinkKind) => (k === "pr" ? "PR をリンク" : "Issue をリンク");
export const kindLabel = (k: LinkKind) => (k === "pr" ? "PR" : "Issue");

export const stateLabel: Record<LinkState, string> = {
  open: "Open",
  draft: "Draft",
  merged: "Merged",
  closed: "Closed",
};

export function reviewLabel(r: LinkReview): string {
  return { none: "", pending: "レビュー待ち", approved: "承認済み", changes_requested: "修正依頼" }[
    r
  ];
}

export function checksLabel(c: LinkChecks): string {
  return { "": "", success: "CI 成功", failure: "CI 失敗", pending: "CI 実行中" }[c];
}

// showsProgress は、レビューと CI を出す行か(開いている PR だけ)。
export const showsProgress = (r: LinkRow) =>
  r.kind === "pr" && (r.state === "open" || r.state === "draft");

export const linkText = (r: LinkTarget & { title: string }) => `${r.repo}#${r.number} — ${r.title}`;

export const linkUrl = (r: LinkTarget) =>
  `https://github.com/${r.repo}/${r.kind === "pr" ? "pull" : "issues"}/${r.number}`;
