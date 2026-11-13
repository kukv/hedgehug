import type { HistoryPort } from "../../port/history";
import { type Readable, readonly, writable } from "../../reactive/store";
import type { LeaveGuard } from "../drafts/drafts";
import { formatRoute, parseRoute, type Route } from "./route";

// 画面の移り方。go は履歴に積み、replace は積まない(01「画面のアドレス」の積む・積まない)。
// guard があり、書きかけがあるときは、go と戻る・進むは「入力を破棄」の確認が「破棄」のときだけ移る(02「説明」)。
export interface Navigation {
  route: Readable<Route>;
  go(route: Route): void;
  replace(route: Route): void;
  // start は、戻る・進むを受け始め、止める関数を返す。
  start(): () => void;
}

export function createNavigation(history: HistoryPort, guard?: LeaveGuard): Navigation {
  const route = writable<Route>(parseRoute(history.path()));
  // shown は、いま出している画面のアドレス。戻る・進むを断ったとき、ここへ戻す。
  let shown = history.path();
  const show = (path: string) => {
    shown = path;
    route.set(parseRoute(path));
  };
  return {
    route: readonly(route),
    go(next) {
      const path = formatRoute(next);
      // いまと同じ画面へは移らない(履歴を積まず、画面を移ったときの動きもしない)。
      if (path === history.path()) return;
      const move = () => {
        history.push(path);
        show(path);
      };
      if (!guard?.anyDirty()) return move();
      void guard.confirmLeave().then((ok) => {
        if (ok) move();
      });
    },
    replace(next) {
      const path = formatRoute(next);
      history.replace(path);
      show(path);
    },
    start() {
      return history.onPop(() => {
        const path = history.path();
        if (!guard?.anyDirty()) return show(path);
        // ブラウザのアドレスはもう変わっている。断ったら、いまの画面のアドレスを積み直して留まる。
        void guard.confirmLeave().then((ok) => {
          if (ok) show(path);
          else history.push(shown);
        });
      });
    },
  };
}
