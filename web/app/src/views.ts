import { navigation } from "./wire";
import { views, viewsPort } from "./wireBase";

// 保存したビューの組み立て(E3b)。口と一覧は、wire.ts が画面のアドレスの解決に使うので wireBase.ts で作り(wire.ts を
// 読むと循環する)、ここで再公開する。
export { views, viewsPort };

// openView は、そのビューの画面へ移る(履歴に積む)。移れたかを返す(「入力を破棄」の確認で断られたら false)。
export function openView(id: string): Promise<boolean> {
  return navigation.go({ name: "view", id });
}
