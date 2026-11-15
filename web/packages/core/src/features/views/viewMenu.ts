import type { Navigation } from "../router/navigation";
import type { Toasts } from "../toasts/toasts";
import type { Views } from "./views";

// ビューの画面の見出しのメニュー(E3b-13)。項目と、複製・削除のあとの流れ。

export interface ViewMenuItem {
  id: "rename" | "duplicate" | "delete";
  label: string;
}

export const viewMenuItems: ViewMenuItem[] = [
  { id: "rename", label: "名前を変更…" },
  { id: "duplicate", label: "複製" },
  { id: "delete", label: "削除…" },
];

// deleteViewBody は、削除の確認の本文。
export function deleteViewBody(name: string): string {
  return `ビュー「${name}」を削除します。アイテムは消えません。元に戻せません。`;
}

// duplicateView は、ビューを複製し、できたらそのビューの画面へ移る。断り・失敗(トーストは views が出した)は何もしない。
export async function duplicateView(
  deps: { views: Pick<Views, "duplicate">; open(id: string): Promise<unknown> },
  id: string,
): Promise<void> {
  const copy = await deps.views.duplicate(id);
  if (copy !== null) await deps.open(copy.id);
}

// deleteView は、ビューを削除し、できたらトーストを出してホームへ移る(履歴には積まない)。
export async function deleteView(
  deps: { views: Pick<Views, "remove">; toasts: Toasts; navigation: Pick<Navigation, "replace"> },
  id: string,
): Promise<void> {
  if (!(await deps.views.remove(id))) return;
  deps.toasts.show("ビューを削除しました");
  deps.navigation.replace({ name: "home" });
}
