import type { Readable } from "../../reactive/store";
import { dayOf } from "./date";

// createToday は、now を「日付が替わったときだけ」渡す今日の値にする。購読すると今の値をすぐ渡す。
// 期限の表示(ツリー・表・概要・マイタスク)とマイタスクのまとめ方が描き直しに使い、日付の変わり目に日付で決まる結果を
// 読み直す(wire.ts)のにも使う。
export function createToday(now: Readable<Date>): Readable<Date> {
  return {
    subscribe(run) {
      let last: string | null = null;
      return now.subscribe((d) => {
        const day = dayOf(d);
        if (day === last) return;
        last = day;
        run(d);
      });
    },
  };
}

// onDayChange は、今日の値が替わるたびに run を呼ぶ。最初の値では呼ばない。戻り値は購読を止める関数。
export function onDayChange(today: Readable<Date>, run: () => void): () => void {
  let first = true;
  return today.subscribe(() => {
    if (first) {
      first = false;
      return;
    }
    run();
  });
}
