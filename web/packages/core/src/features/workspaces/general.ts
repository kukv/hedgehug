import type { WorkspaceUsage } from "../../model/workspace";

// 「一般」の接頭辞の文言(03「接頭辞」)。

export const prefixHelp =
  "Epic・Story・Task・Bug・Step の ID の頭に付きます。変更すると、すべての ID が新しい接頭辞で表示されます(番号はそのままです)。";

// prefixExamples は、接頭辞の欄の例。番号の小さい方から 3 件まで。
// アイテムがなければ空(画面が「アイテムはまだありません」を出す)。
export function prefixExamples(prefix: string, firstNumbers: number[]): string[] {
  return firstNumbers.slice(0, 3).map((n) => `${prefix}-${n}`);
}

// prefixConfirm は、確認ダイアログ「接頭辞を変更」の本文。
export function prefixConfirm(oldPrefix: string, newPrefix: string, usage: WorkspaceUsage): string {
  const first = usage.firstNumbers[0];
  if (usage.items === 0 || first === undefined) {
    return `接頭辞を ${oldPrefix} から ${newPrefix} に変更します。ID が変わるアイテムはありません。旧い接頭辞でも検索できます。`;
  }
  return `${usage.items} 件の ID が ${oldPrefix}-${first} → ${newPrefix}-${first} のように変わります。古い ID で書かれたコメントやリンクは、そのままの文字で残ります。旧い接頭辞でも検索できます。`;
}

// prefixRefusal は、接頭辞の「変更」を押したときの検査(確認ダイアログの前)。作成と同じ規則で、
// 当たらなければ null。others は、同じ Store のほかのワークスペースの接頭辞。
export function prefixRefusal(prefix: string, others: string[]): string | null {
  if (prefix === "") return "接頭辞を入力してください";
  if (!/^[A-Z][A-Z0-9]{0,9}$/.test(prefix)) {
    return "接頭辞は英大文字で始まる 10 文字以内の英大文字と数字にしてください";
  }
  if (others.includes(prefix)) return "この Store には同じ接頭辞のワークスペースがあります";
  return null;
}
