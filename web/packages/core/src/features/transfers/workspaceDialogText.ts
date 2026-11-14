import type { WorkspaceTransferPreview } from "../../model/transfer";

// ワークスペースの設定「一般」の「別の Store へ移動」の区画と確認ダイアログの文言(03「別の Store へ移動」)。
// 仕様にない文言(セレクトの名前・ボタン・外との関連の点・人の点)は B5c-14 の文で、Task 12 で仕様に書き足す。
// 失敗のトーストは、アイテムの移動と同じ文(dialogText.ts の offlineMessage・transferFailedMessage)を使う。

export const workspaceTransferNote =
  "ワークスペースをまるごと、別の git リポジトリへ移動します。移動先には、この Store の履歴は引き継がれません。";
export const workspaceTargetLabel = "移動先の Store";
export const workspaceTransferButton = "移動…";
export const noWorkspaceTargetsMessage = "移せる Store がありません";
export const noWorkspaceTargetsHiddenMessage =
  "移せる Store がありません(非表示の Store は、設定の「Store」で表示に戻してください)";
export const workspaceTransferTitle = "別の Store へ移動";
export const workspaceTransferOk = "移動";
export const workspaceTransferCancel = "キャンセル";
export const workspaceTransferNameLabel = "確認のため、ワークスペースの名前を入力してください";

const outsideRelations =
  "ワークスペースの外にあるアイテムとの関連は、種類を問わず解除されます。ワークスペースの中のアイテム同士の関連は残ります。";

// workspaceTransferLines は、確認ダイアログの本文の行(03 の点の順)。人がいなければ人の行を、
// remote の注意がなければ最後の行を出さない。
export function workspaceTransferLines(name: string, p: WorkspaceTransferPreview): string[] {
  let count = `プロジェクト ${p.projects} 件・アイテム ${p.items} 件を移動します。`;
  if (p.deleted > 0) count += `削除済みの ${p.deleted} 件も一緒に移ります。`;
  const lines = [
    `「${name}」を ${p.source} から ${p.destination} へ移動します。`,
    count,
    outsideRelations,
  ];
  if (p.people.length > 0) {
    const names = p.people.map((m) => `${m.name}さん`).join("・");
    lines.push(`${names}の ${p.people.length} 人が、移動先のメンバーに追加されます`);
  }
  if (p.remoteNotice)
    lines.push("移動先に remote がないため、ほかの人の Store からは見えなくなります。");
  return lines;
}

// workspaceMovedMessage は、移したあとのトースト。storeName は移動先の Store 名。
export const workspaceMovedMessage = (storeName: string) => `${storeName} へ移動しました`;
