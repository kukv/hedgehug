import type { NotCarried, ProjectTransferPreview } from "../../model/transfer";

// プロジェクトの設定の「別のワークスペースへ移動」の区画と確認ダイアログの文言(02 **移動**)。
// 仕様にない文言(区画の説明文・セレクトの名前・タイトル・hiddenFixed などの行)は A5b-18 の文で、Task 12 で仕様に書き足す。
// 失敗のトーストは、アイテムの移動と同じ文(dialogText.ts の offlineMessage・transferFailedMessage)を使う。

export const projectTransferNote =
  "このプロジェクトと、その下のすべてのアイテム(削除済みを含む)を、別のワークスペースへ移します。";
export const projectTargetLabel = "移動先のワークスペース";
export const noProjectTargetsMessage = "移せるワークスペースがありません";
export const projectTransferTitle = "プロジェクトを移動";
export const projectTransferOk = "移動";
export const notCarriedHeading = "移らないもの";

// projectTransferIntro は、確認ダイアログの本文。source・destination は A5b-10 の {移動元}・{移動先} の文。
export function projectTransferIntro(title: string, p: ProjectTransferPreview): string {
  let s = `「${title}」を ${p.source} から ${p.destination} へ移動します。${p.items} 件のアイテムは ${p.prefix} の番号になります。`;
  if (p.deleted > 0) s += `削除済みの ${p.deleted} 件も一緒に移ります。`;
  if (p.crossStore)
    s +=
      "移さないアイテムとの関連を解除します。移すもの同士の関連は残ります。移動先のメンバーにいない人は、移動先のメンバーに加わります。";
  if (p.remoteNotice) s += "移動先に remote がないため、ほかの人の Store からは見えなくなります。";
  if (p.hasRules)
    s +=
      "このプロジェクトのルールは、移動先の同じ名前のステータス・テンプレートに合わせて移ります。";
  return s;
}

// notCarriedLine は、見出し「移らないもの」の下の 1 行(A5b-8)。行を出さないもの(動かさない指定が 0 件)は null。
export function notCarriedLine(n: NotCarried): string | null {
  switch (n.kind) {
    case "hidden":
      return `使わないステータス「${n.name}」(移動先にありません)`;
    case "hiddenFixed":
      return `使わないステータス「${n.name}」(移動先では使わないことにできません)`;
    case "workflow":
      return `遷移のチェック「${n.name}」(移動先に${n.missing.map((m) => `「${m}」`).join("")}がありません)`;
    case "template":
      return `使うテンプレート「${n.name}」(移動先にありません)`;
    case "templateDefault":
      return `${n.name} の既定のテンプレート「${n.missing[0] ?? ""}」(移動先にありません)`;
    case "disabled":
      return n.count > 0
        ? `ルールを動かさない指定 ${n.count} 件(移動先のルールには引き継ぎません)`
        : null;
  }
}

// replacementLabel は、「ステータスの置き換え」の行の見出し。ルールだけが使うもの(0 件)は件数を出さない。
export const replacementLabel = (name: string, count: number) =>
  count > 0 ? `${name}(${count} 件)` : name;

// replacementSelectLabel は、「ステータスの置き換え」の行のセレクトの名前(02「別のワークスペースと別の Store への移動」)。
export const replacementSelectLabel = (name: string) => `${name} の置き換え先`;

// projectMovedMessage は、移したあとのトースト。destination は A5b-10 の {移動先} の文。
export const projectMovedMessage = (destination: string) => `${destination} へ移動しました`;
