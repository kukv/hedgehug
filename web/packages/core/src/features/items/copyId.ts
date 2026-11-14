import type { Toasts } from "../toasts/toasts";

// 「ID をコピー」(02「上から順の構成」)。種類の行のボタンとアイテムのメニューの項目が同じ結果を出す(A4b-17)。
// クリップボードは DOM なので、app が write を渡す。

// コピーの結果のトーストの文言。サインインのダイアログのコードのコピーでも使う(detail.ts が 300 行なのでここに置く)。
export const copiedMessage = (id: string) => `コピーしました: ${id}`;
export const copyFailedMessage = (id: string) =>
  `コピーできませんでした。${id} を選択してコピーしてください`;

export interface CopyIdDeps {
  write(text: string): Promise<void>;
  toasts: Toasts;
}

// createCopyId は、ID の表記(HH-3)をコピーして、結果をトーストで出す関数を作る。閲覧のみの Store でも使える。
export function createCopyId(deps: CopyIdDeps): (displayId: string) => Promise<void> {
  return async (displayId) => {
    try {
      await deps.write(displayId);
      deps.toasts.show(copiedMessage(displayId));
    } catch {
      deps.toasts.show(copyFailedMessage(displayId), { error: true });
    }
  };
}
