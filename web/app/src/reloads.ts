// 画面への反映(06「画面への反映」)で読み直す、wire.ts の外で作った状態。wire.ts が 300 行なので、
// 読み直しの関数はここに登録し、wire.ts の reflect が呼ぶ。
export const reloads = new Set<() => Promise<unknown>>();
// githubReloads は、github の知らせのときだけ読み直すもの(アカウントの一覧。認証エラーの確かめ直しで戻る。D4a-18)。
export const githubReloads = new Set<() => Promise<unknown>>();
