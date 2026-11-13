import type { TransferPreview } from "../../model/transfer";

// 「別のワークスペースへ移動…」のダイアログとトーストの文言(02「別のワークスペースと別の Store への移動」、A5a-12)。
// 仕様にない文言(見出しの下の各行・「移動できませんでした」)は A5a-23 で仕様に書き足す。

export const noTargetsMessage =
  "移せるワークスペースがありません(プロジェクトのある、書き込めるワークスペースが要ります)";
export const noEpicMessage = "このプロジェクトには Epic がありません";
export const changesHeading = "移動先で変わること";
export const replacementsHeading = "ステータスの置き換え";
// 断りでも、オフラインでもない失敗のときの文言(A4b の失敗のトーストと同じ扱い)。
export const transferFailedMessage = "移動できませんでした";
export const offlineMessage = (storeName: string) =>
  `オフラインのため ${storeName} へ移動できません`;

// transferredMessage は、移したあとのトースト。空になった項目があれば続ける。
export function transferredMessage(oldId: string, display: string, emptied: string[]): string {
  const done = `${oldId} は ${display} になりました`;
  return emptied.length === 0 ? done : `${done}。合う値がなく空になった項目: ${emptied.join("・")}`;
}

// transferIntro は、ダイアログの冒頭の文。below は下の削除されていないもの、deleted は下の削除済みのものの件数。
export function transferIntro(
  displayId: string,
  title: string,
  below: number,
  deleted: number,
): string {
  const head =
    below > 0
      ? `${displayId} ${title} と、その下の ${below} 件を移動します。`
      : `${displayId} ${title} を移動します。`;
  return deleted > 0 ? `${head}削除済みの ${deleted} 件も一緒に移ります。` : head;
}

// changeRows は、見出し「移動先で変わること」の下の行。別の Store の行は値の行のあと、転送記録の行はいつも最後。
export function changeRows(p: TransferPreview): string[] {
  const rows = [
    `ID は ${p.nextDisplay} からの番号に振り直されます`,
    "スプリントとリリースは外されます",
    "優先度・関連システムなどは、同じ名前の値に合わせます。合う値がなければ空になります。ステータスは、同じ名前で移動先のプロジェクトが使うものに合わせ、合わないものは「ステータスの置き換え」で選んだステータスにします",
  ];
  if (p.crossStore) {
    rows.push(
      "移さないアイテムとの関連は解除されます。移すもの同士の関連は残ります",
      "移動先のメンバーにいない人は、移動先のメンバーに加わります",
    );
  }
  if (p.remoteNotice)
    rows.push("移動先に remote がないため、ほかの人の Store からは見えなくなります。");
  rows.push("移動元に転送の記録(移した日時を含む)を残し、古い ID からも辿れます");
  return rows;
}
