import type { Item } from "@hedgehug/core";

// 画面への反映(06「画面への反映」)で読み直す、wire.ts の外で作った状態。wire.ts が 300 行なので、
// 読み直しの関数はここに登録し、wire.ts の reflect が呼ぶ。
export const reloads = new Set<() => Promise<unknown>>();
// githubReloads は、github の知らせのときだけ読み直すもの(アカウントの一覧。認証エラーの確かめ直しで戻る。D4a-18)。
export const githubReloads = new Set<() => Promise<unknown>>();
// propertySaved は、詳細パネルでプロパティを保存できたときに呼ぶもの(バックログの消えるときのトースト。F2a-11)。
// planning.ts は wire.ts を読むので、wire.ts から planning.ts を読まずに、ここに登録する。
export const propertySaved = new Set<(storeId: string, propertyId: string, item: Item) => void>();
