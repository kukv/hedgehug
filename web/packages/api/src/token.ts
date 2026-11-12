// サーバーが出す URL は http://127.0.0.1:<番号>/#token=<トークン>。トークンをハッシュに置くのは、
// サーバーのログと Referer に残さないため(土台の計画の決定 7)。
export function tokenFromHash(hash: string): string | null {
  const token = new URLSearchParams(hash.replace(/^#/, "")).get("token");
  return token ? token : null;
}

// tokenKey は、sessionStorage に置くトークンの鍵。
export const tokenKey = "hedgehug.token";

// TokenStorage は、sessionStorage のうち使う部分(テストで差し替える)。
export interface TokenStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

// rememberToken は、起動時に一度呼び、使うトークンを返す(A2 の計画の決定 5)。
// アドレスのハッシュにトークンがあれば storage に移し、replace でアドレスからハッシュを消す。
// なければ(同じタブの再読み込み)storage から読む。どちらにもなければ空。
export function rememberToken(
  location: { hash: string; pathname: string; search: string },
  storage: TokenStorage,
  replace: (url: string) => void,
): string {
  const fromHash = tokenFromHash(location.hash);
  if (fromHash) {
    storage.setItem(tokenKey, fromHash);
    replace(location.pathname + location.search);
    return fromHash;
  }
  return storage.getItem(tokenKey) ?? "";
}

// newTabUrl は、アプリの中の操作で新しいタブに開くアドレス。トークンをハッシュに付ける
// (リンクの href には付けない。右クリックの「新しいタブで開く」はトークンのない URL になる。08)。
export function newTabUrl(path: string, token: string): string {
  return `${path}#token=${encodeURIComponent(token)}`;
}
