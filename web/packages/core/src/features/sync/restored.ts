// restoredToast は、別の画面で削除されていたものへの保存で削除を取り消したときのトーストの文。
export function restoredToast(id: string): string {
  return `${id} は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました`;
}
