const labels: Record<string, string> = {
  add: "作成",
  set: "変更",
  edit: "本文の編集",
  mv: "移動",
  rm: "削除",
  restore: "復元",
  purge: "完全削除",
  merge: "同期でマージ",
  repair: "同期で修復",
  comment: "コメント",
  undo: "取り消し",
  init: "Store の作成",
  status: "ステータス変更",
  dep: "依存の変更",
  watch: "ウォッチ",
  unwatch: "ウォッチ解除",
  "move-in": "Store から移動してきた",
  "move-out": "Store へ移動した",
  "purge-moved": "移動の転送の削除",
  "rewrite-history": "履歴の書き換え",
  "revert-overwritten": "上書きの取り消し",
  migrate: "スキーマの移行",
  workspace: "ワークスペースの変更",
  view: "ビューの変更",
  setup: "同期の準備",
  replay: "同期で再適用",
};

// opLabel names the Hedgehug-Op of a commit; an op it does not know is shown as it is.
export function opLabel(op: string | null): string {
  if (op === null || op === "") return "変更";
  return labels[op] ?? op;
}
