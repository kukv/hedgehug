// サーバーが操作を断ったとき(API の 422)。message は仕様の文言で、画面にそのまま出す(A2 の計画の決定 13)。
export class Refused extends Error {
  override name = "Refused";
}

// 指したものがないとき(API の 404)。
export class NotFound extends Error {
  override name = "NotFound";
}

// 別の Store への移動で、移動元から取り除けず、移動先の保存の取り消しもできなかったとき(移動の API の 409。
// アイテムは両方の Store にある)。message は仕様の文言で、画面にそのまま出す(02「保存」)。
export class TransferUndoFailed extends Error {
  override name = "TransferUndoFailed";
}
