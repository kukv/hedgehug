import { get, type Readable } from "../../reactive/store";
import type { Navigation } from "../router/navigation";
import type { Resolved } from "../router/resolve";
import { formatRoute, parseRoute } from "../router/route";
import type { ScreenStates } from "../screenstate/screenStates";

// 最後に開いていた画面(E4a)。出した画面のアドレスを画面の状態に覚え、画面を指さないアドレス(/)で開いたときは、
// そのアドレスに替える(履歴に積まない)。覚えていなければマイタスク。ホームとアイテムの入口は覚えない(アイテムの
// 入口はプロジェクトのツリーに替わるので、替わった先を覚える)。開き直した画面がもうなくマイタスクに解決したときは、
// アドレスもマイタスクに替える。

export const lastScreenKey = "last-screen";

export interface LastScreen {
  // start は、出す画面とアドレスの変化を受け始め、止める関数を返す。
  start(): () => void;
}

export interface LastScreenDeps {
  navigation: Navigation;
  current: Readable<Resolved | null>;
  screenStates: ScreenStates;
}

export function createLastScreen(deps: LastScreenDeps): LastScreen {
  // follow は、開き直したアドレスの次の解決を 1 回だけ見る。覚えた画面が(転送の記録もなく)消えていてマイタスクに
  // 解決したときは、アドレスもマイタスクに替える(消えたアドレスを残さない)。転送の記録があるときは、screen.ts が
  // 理由のトーストを出してアドレスも替えるので、アドレスがまだ path のときだけ替える。いまの値(開き直す前の画面)は見ない。
  const follow = (path: string) => {
    let first = true;
    let done = false;
    const stop = deps.current.subscribe((r) => {
      if (first || done || r === null || r.name === "home") return;
      done = true;
      stop();
      if (r.name === "mytasks" && formatRoute(get(deps.navigation.route)) === path)
        deps.navigation.replace({ name: "mytasks" });
    });
    first = false;
  };
  const restore = async () => {
    const saved = await deps.screenStates.read<{ path: string } | null>(lastScreenKey, null);
    // 読んでいるあいだにほかの画面へ移っていたら、替えない。
    if (get(deps.navigation.route).name !== "home") return;
    const to = typeof saved?.path === "string" ? parseRoute(saved.path) : null;
    // 覚えたアドレスが / のときもマイタスクにする(home に替えると、また読みにいく)。
    deps.navigation.replace(to && to.name !== "home" ? to : { name: "mytasks" });
    if (to && to.name !== "home" && to.name !== "mytasks") follow(formatRoute(to));
  };
  return {
    start() {
      const stopCurrent = deps.current.subscribe((r) => {
        if (r === null || r.name === "home" || r.name === "item") return;
        // マイタスクに解決したときは、打ったままの解決できないアドレスではなく、マイタスクのアドレスを覚える。
        const route = r.name === "mytasks" ? r : get(deps.navigation.route);
        deps.screenStates.write(lastScreenKey, { path: formatRoute(route) });
      });
      const stopRoute = deps.navigation.route.subscribe((route) => {
        if (route.name === "home") void restore();
      });
      return () => {
        stopCurrent();
        stopRoute();
      };
    },
  };
}
