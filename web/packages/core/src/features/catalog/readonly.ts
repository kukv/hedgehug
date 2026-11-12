// 閲覧のみの Store の文言(01「閲覧のみの Store」)。

// readOnlyNote は、無効にした入力欄・ボタンに添える文言。
export const readOnlyNote = "閲覧のみの Store です";

// readOnlyBand は、ワークスペース・プロジェクトの画面と詳細パネルの上部に出す帯の文。
export function readOnlyBand(storeName: string): string {
  return `${storeName} は閲覧のみの Store です。閲覧はできますが、作成・変更・削除はできません。`;
}
