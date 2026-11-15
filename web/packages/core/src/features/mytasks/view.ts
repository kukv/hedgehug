// マイタスク(04「マイタスク」)の文言。

export const myTasksTitle = "マイタスク";
export const myTasksNote = "あなたが担当しているタスク";
export const myTasksPlaceholder = "絞り込み 例: priority:high";

// myTasksEmpty は、行がないときの文。baseCount は、絞り込む前の件数。hasErrors は、絞り込みに誤りがあるか
// (誤りがあるときは、絞り込む前の件数が 0 でも絞り込みの文にする。E4a-8)。
export function myTasksEmpty(
  baseCount: number,
  hasErrors: boolean,
): { title: string; body: string } {
  if (baseCount === 0 && !hasErrors) {
    return {
      title: "担当しているタスクはありません",
      body: "新しく割り当てられると、ここと受信箱に出ます。",
    };
  }
  return { title: "一致するものはありません", body: "絞り込みを変えてみてください。" };
}

// myTasksBadge は、サイドバーのバッジの文。
export const myTasksBadge = (n: number): string => `未完了 ${n} 件`;
