import type { SyncPort } from "../../port/sync";

// タブの知らせ(決定 C2-18・C2-18b、C2 画面の決定 S3・S4)。サーバーは知らせを期限で捨てるので、続くあいだは
// 10 秒ごとに送り直す。送る失敗は捨てる(次の送り直しで届く)。

export const resendMs = 10_000;

export interface TabSignalsDeps {
  port: SyncPort;
  tab: string;
  held(): boolean;
  visible(): boolean;
  online(): boolean;
  // now は、いまの時刻(ミリ秒)。
  now(): number;
}

export interface TabSignals {
  // check は、いまの状態を見て、要る知らせを送る。app が 1 秒ごとと、状態が変わったときに呼ぶ。
  check(): void;
}

// signal は、1 つの真偽の知らせ。on のあいだは 10 秒ごとに送り、off になったら 1 回送る。
function signal(send: (on: boolean) => Promise<void>) {
  let on = false;
  let sentAt = Number.NEGATIVE_INFINITY;
  return (next: boolean, now: number) => {
    if (next && (!on || now - sentAt >= resendMs)) {
      sentAt = now;
      void send(true).catch(() => {});
    } else if (!next && on) {
      void send(false).catch(() => {});
    }
    on = next;
  };
}

export function createTabSignals(deps: TabSignalsDeps): TabSignals {
  const hold = signal((on) => deps.port.hold(deps.tab, on));
  const offline = signal((on) => deps.port.network(deps.tab, !on));
  return {
    check() {
      const now = deps.now();
      hold(deps.visible() && deps.held(), now);
      offline(!deps.online(), now);
    },
  };
}
