import {
  createNameDialog,
  createSearch,
  get,
  type Search,
  saveAsView,
  type ViewContent,
  viewScreenKey,
} from "@hedgehug/core";
import { screenStates, searchPort, searchSelection } from "./search";
import { navigation } from "./wire";
import { timer, toasts, views, viewsPort } from "./wireBase";

// 保存したビューの組み立て(E3b)。口と一覧は、wire.ts が画面のアドレスの解決に使うので wireBase.ts で作り(wire.ts を
// 読むと循環する)、ここで再公開する。
export { views, viewsPort };

// openView は、そのビューの画面へ移る(履歴に積む)。移れたかを返す(「入力を破棄」の確認で断られたら false)。
export function openView(id: string): Promise<boolean> {
  return navigation.go({ name: "view", id });
}

// createViewSearch は、ビューの画面の検索の流れ(E3b-3)。画面の内容は画面の状態の鍵 view:{id} に置き、鍵に何もない
// ときは一覧のいまの保存した内容(開閉は空)で始める。選択は検索の画面と同じものを使う(E3b-14)。
export function createViewSearch(id: string): Search {
  return createSearch({
    port: searchPort,
    screenStates,
    timer,
    toasts,
    selection: searchSelection,
    screen: viewScreenKey(id),
    fallback: () => get(views.list).find((v) => v.id === id)?.content ?? null,
  });
}

// ダイアログ「ビューとして保存」(E3b-10)。検索の画面のボタンとパレットが openSaveView で開く。
export const saveView = createNameDialog();

export function openSaveView(content: ViewContent): void {
  saveView.open((name) => saveAsView({ views, toasts, open: openView }, name, content));
}

// 画面を移ったら、出しているダイアログを閉じる(01「画面の移り方」)。
navigation.route.subscribe(() => saveView.close());
