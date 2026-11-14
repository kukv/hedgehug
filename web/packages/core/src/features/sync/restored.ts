// restoredToast は、別の画面で削除されていたものへの保存で削除を取り消したときのトーストの文。
export function restoredToast(id: string): string {
  return `${id} は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました`;
}

// restoredNameToast は、設定の値(プロパティ・値・テンプレート・自動化のルール)の削除を取り消したときの文。
export function restoredNameToast(name: string): string {
  return `「${name}」は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました`;
}

// restoredCommentToast は、自分のコメントの削除を取り消したときの文。
export function restoredCommentToast(): string {
  return "コメントは別の画面で削除されていましたが、変更を保存したため、削除を取り消しました";
}
