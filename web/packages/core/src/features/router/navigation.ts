import type { HistoryPort } from "../../port/history";
import { type Readable, readonly, writable } from "../../reactive/store";
import { formatRoute, parseRoute, type Route } from "./route";

// 画面の移り方。go は履歴に積み、replace は積まない(01「画面のアドレス」の積む・積まない)。
export interface Navigation {
  route: Readable<Route>;
  go(route: Route): void;
  replace(route: Route): void;
  // start は、戻る・進むを受け始め、止める関数を返す。
  start(): () => void;
}

export function createNavigation(history: HistoryPort): Navigation {
  const route = writable<Route>(parseRoute(history.path()));
  return {
    route: readonly(route),
    go(next) {
      const path = formatRoute(next);
      // いまと同じ画面へは移らない(履歴を積まず、画面を移ったときの動きもしない)。
      if (path === history.path()) return;
      history.push(path);
      route.set(parseRoute(path));
    },
    replace(next) {
      const path = formatRoute(next);
      history.replace(path);
      route.set(parseRoute(path));
    },
    start() {
      return history.onPop(() => route.set(parseRoute(history.path())));
    },
  };
}
