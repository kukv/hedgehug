import type { SyncPort } from "../../port/sync";

// タブの知らせ(決定 C2-18・C2-18b、C2 画面の決定 S3・S4)。サーバーは知らせを期限で捨てるので、続くあいだは
// 10 秒ごとに送り直す。on の送りの失敗は捨てる(次の送り直しで届く)。

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

// signal は、1 つの真偽の知らせ。on のあいだは 10 秒ごとに送り、off になったら送る。off の送りに失敗したら、
// 成功するまで次の check で送り直す(取りこぼすと、サーバーは期限が切れるまで知らせが続くと見る。決定 G1-12)。
function signal(send: (on: boolean) => Promise<void>) {
  let on = false;
  let sentAt = Number.NEGATIVE_INFINITY;
  let releasing = false; // off の知らせが、まだ届いていない
  let inFlight = false;
  let offs = 0; // off になった回数。送っているあいだに次の off があったら、届いたことにしない
  const release = () => {
    if (inFlight) return;
    inFlight = true;
    const mine = offs;
    void send(false)
      .then(() => {
        if (offs === mine) releasing = false;
      })
      .catch(() => {})
      .finally(() => {
        inFlight = false;
      });
  };
  return (next: boolean, now: number) => {
    if (next && (!on || now - sentAt >= resendMs)) {
      sentAt = now;
      releasing = false;
      void send(true).catch(() => {});
    } else if (!next && on) {
      releasing = true;
      offs += 1;
    }
    if (!next && releasing) release();
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
