// サーバーが出す URL は http://127.0.0.1:<番号>/#token=<トークン>。トークンをハッシュに置くのは、
// サーバーのログと Referer に残さないため(この計画の決定 7)。
export function tokenFromHash(hash: string): string | null {
  const token = new URLSearchParams(hash.replace(/^#/, "")).get("token");
  return token ? token : null;
}
