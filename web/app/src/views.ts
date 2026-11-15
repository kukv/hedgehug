import { createNameDialog, saveAsView, type ViewContent } from "@hedgehug/core";
import { navigation } from "./wire";
import { toasts, views, viewsPort } from "./wireBase";

// 保存したビューの組み立て(E3b)。口と一覧は、wire.ts が画面のアドレスの解決に使うので wireBase.ts で作り(wire.ts を
// 読むと循環する)、ここで再公開する。
export { views, viewsPort };

// openView は、そのビューの画面へ移る(履歴に積む)。移れたかを返す(「入力を破棄」の確認で断られたら false)。
export function openView(id: string): Promise<boolean> {
  return navigation.go({ name: "view", id });
}

// ダイアログ「ビューとして保存」(E3b-10)。検索の画面のボタンとパレットが openSaveView で開く。
export const saveView = createNameDialog();

export function openSaveView(content: ViewContent): void {
  saveView.open((name) => saveAsView({ views, toasts, open: openView }, name, content));
}

// 画面を移ったら、出しているダイアログを閉じる(01「画面の移り方」)。
navigation.route.subscribe(() => saveView.close());
