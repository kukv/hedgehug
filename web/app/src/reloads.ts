import type { Item } from "@hedgehug/core";

// 画面への反映(06「画面への反映」)で読み直す、wire.ts の外で作った状態。wire.ts が 300 行なので、
// 読み直しの関数はここに登録し、wire.ts の reflect が呼ぶ。
export const reloads = new Set<() => Promise<unknown>>();
// githubReloads は、github の知らせのときだけ読み直すもの(アカウントの一覧。認証エラーの確かめ直しで戻る。D4a-18)。
export const githubReloads = new Set<() => Promise<unknown>>();
// propertySaved は、詳細パネルでプロパティを保存できたときに呼ぶもの(バックログの消えるときのトースト。F2a-11)。
// planning.ts は wire.ts を読むので、wire.ts から planning.ts を読まずに、ここに登録する。
export const propertySaved = new Set<(storeId: string, propertyId: string, item: Item) => void>();
// inboxReloads は、inbox の知らせのときだけ読み直すもの(受信箱の未読数と一覧。C4a-13)。
export const inboxReloads = new Set<() => Promise<unknown>>();
// itemChanges は、アイテムを変えたあとの読み直し(wireActions.ts の itemRefresh.afterChange)で読み直すもの(検索のツリー。
// 削除・移動・並べ替えのあと、Store が変わった知らせを待たずに描き直す)。
export const itemChanges = new Set<() => Promise<unknown>>();
// syncReloads は、sync の知らせのときだけ読み直すもの(アクティビティの同期の行。C4d-15)。
export const syncReloads = new Set<() => Promise<unknown>>();
