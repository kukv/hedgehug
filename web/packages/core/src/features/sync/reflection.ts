import type { SyncEventKind } from "../../model/sync";
import type { SyncPort, Watching } from "../../port/sync";
import type { Timer } from "../toasts/toasts";

// 画面への反映(06「画面への反映」「反映を待つとき」。決定 S10〜S12)。出来事を種類ごとに溜め、待つ条件に
// なく、見えているときに 1 回で apply する。待っているあいだは 0.5 秒ごとに確かめる。

export interface ReflectionDeps {
  port: SyncPort;
  timer: Timer;
  // held は、このタブが待つ条件(ダイアログ・メニュー・入力欄のフォーカスほか)にあるか。
  held(): boolean;
  // visible は、このタブが見えているか。
  visible(): boolean;
  // apply は、溜めた種類の画面を読み直す。順は store、sync。
  apply(kinds: SyncEventKind[]): Promise<void>;
}

export interface Reflection {
  // start は、出来事を受け始め(見えていれば、つなぐ)、止める関数を返す。
  start(): () => void;
  // now は、待つ条件を問わず、すべてをすぐに読み直す(手動の同期のあと)。
  now(): Promise<void>;
  // wake は、見えるか・待つ条件が変わったかもしれないときに呼ぶ。見えていればつなぎ、見えていなければ
  // 切り(決定 S22)、溜めたものを確かめる。
  wake(): void;
}

const retryMs = 500;
const firstBackoff = 1000;
const maxBackoff = 10_000;
const order: SyncEventKind[] = ["store", "sync"];

export function createReflection(deps: ReflectionDeps): Reflection {
  const pending = new Set<SyncEventKind>();
  let retry: unknown;
  let applying = false;

  async function flush() {
    if (retry !== undefined) {
      deps.timer.clear(retry);
      retry = undefined;
    }
    if (pending.size === 0 || applying) return;
    if (deps.held() || !deps.visible()) {
      retry = deps.timer.set(() => void flush(), retryMs);
      return;
    }
    const kinds = order.filter((k) => pending.has(k));
    pending.clear();
    applying = true;
    try {
      await deps.apply(kinds);
    } finally {
      applying = false;
    }
    if (pending.size > 0) void flush();
  }

  function add(...kinds: SyncEventKind[]) {
    for (const k of kinds) pending.add(k);
    void flush();
  }

  let stopped = true;
  let current: { w: Watching; dropped: boolean } | undefined;
  let again: unknown;
  let backoff = firstBackoff;

  // connect は、流れがなく、つなぎ直しを待っていなければ、つなぐ。切れたら、見えているあいだだけつなぎ直す。
  function connect() {
    if (stopped || current || again !== undefined) return;
    const c = {
      w: deps.port.watch({
        open: () => {
          backoff = firstBackoff;
          add(...order);
        },
        event: (k) => add(k),
      }),
      dropped: false,
    };
    current = c;
    void (async () => {
      try {
        await c.w.done;
      } catch {
        // つなげなかった・切れた。下でつなぎ直す。
      }
      if (current === c) current = undefined;
      if (c.dropped || stopped || !deps.visible()) return;
      again = deps.timer.set(() => {
        again = undefined;
        connect();
      }, backoff);
      backoff = Math.min(backoff * 2, maxBackoff);
    })();
  }

  function disconnect() {
    if (current) {
      current.dropped = true;
      current.w.stop();
    }
    current = undefined;
    if (again !== undefined) deps.timer.clear(again);
    again = undefined;
  }

  return {
    start() {
      stopped = false;
      if (deps.visible()) connect();
      return () => {
        stopped = true;
        disconnect();
        if (retry !== undefined) deps.timer.clear(retry);
        retry = undefined;
      };
    },
    async now() {
      for (const k of order) pending.delete(k);
      await deps.apply([...order]);
    },
    wake() {
      if (deps.visible()) connect();
      else disconnect();
      void flush();
    },
  };
}
