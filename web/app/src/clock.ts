import { createToday, readonly, writable } from "@hedgehug/core";

// app の時計。wire.ts が 300 行なので、ここに置く。

// syncNow は、相対時間と 24 時間の判定に使う時刻。30 秒ごとと、画面に戻ったときに進める(決定 S7、E4a-16)。
const now = writable(new Date());
export const syncNow = readonly(now);
// today は、日付が替わったときだけ値が替わる今日。期限の表示とマイタスクのまとめ方に使う(E4a-16)。
export const today = createToday(syncNow);

// tickClock は、時計を今の時刻に進める。
export function tickClock() {
  now.set(new Date());
}

setInterval(tickClock, 30_000);
