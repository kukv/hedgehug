// サーバーが操作を断ったとき(API の 422)。message は仕様の文言で、画面にそのまま出す(A2 の計画の決定 13)。
export class Refused extends Error {
  override name = "Refused";
}

// 指したものがないとき(API の 404)。
export class NotFound extends Error {
  override name = "NotFound";
}
