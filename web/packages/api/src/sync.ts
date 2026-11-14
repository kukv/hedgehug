import type { SyncEventKind, SyncPort, Watching } from "@hedgehug/core";
import { call } from "./errors";
import { ApiError, authHeader } from "./fetcher";
import {
  getWatchEventsUrl,
  holdSync,
  listSyncStatus,
  syncAll,
  syncNetwork,
  syncStore,
} from "./generated";

// 同期の口の実装。出来事は、生成される watchEvents() が流れの終わりまで解決しないので、fetch で読み続ける
// (決定 C2-20、C2 の引き継ぎ)。

const kindOf = (name: string): SyncEventKind | null =>
  name === "store" || name === "sync" || name === "github" || name === "inbox" ? name : null;

async function read(on: Parameters<SyncPort["watch"]>[0], signal: AbortSignal): Promise<void> {
  let res: Response;
  try {
    res = await fetch(getWatchEventsUrl(), {
      headers: { Authorization: authHeader(), Accept: "text/event-stream" },
      cache: "no-store",
      signal,
    });
  } catch (e) {
    if (signal.aborted) return;
    throw e;
  }
  if (!res.ok || !res.body) throw new ApiError(res.status, `${res.status} ${res.statusText}`);
  on.open();
  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
  let buf = "";
  let event = "";
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) return;
      buf += value;
      let nl = buf.indexOf("\n");
      while (nl >= 0) {
        const line = buf.slice(0, nl);
        buf = buf.slice(nl + 1);
        if (line.startsWith("event: ")) event = line.slice(7);
        if (line === "") {
          const k = kindOf(event);
          if (k) on.event(k);
          event = "";
        }
        nl = buf.indexOf("\n");
      }
    }
  } catch (e) {
    if (signal.aborted) return;
    throw e;
  }
}

// watch は、止めたら解決する流れを返す(core の Watching)。
function watch(on: Parameters<SyncPort["watch"]>[0]): Watching {
  const abort = new AbortController();
  return { done: read(on, abort.signal), stop: () => abort.abort() };
}

export function createSyncPort(): SyncPort {
  return {
    status: () =>
      call(async () => {
        const body = await listSyncStatus();
        return {
          auto: body.auto,
          stores: body.stores.map((s) => ({
            storeId: s.store,
            name: s.name,
            hidden: s.hidden,
            readOnly: s.readOnly,
            error: s.error,
            unsent: s.unsent,
            lastSuccess: s.lastSuccess === "" ? null : new Date(s.lastSuccess),
          })),
        };
      }),
    syncStore: (storeId) =>
      call(async () => {
        const r = await syncStore(storeId);
        return { result: r.result, message: r.message, mirrorWarning: r.mirrorWarning };
      }),
    syncAll: () =>
      call(async () => {
        const r = await syncAll();
        return { result: r.result, lines: r.lines };
      }),
    hold: (tab, hold) => call(() => holdSync({ tab, hold })),
    network: (tab, online) => call(() => syncNetwork({ tab, online })),
    watch,
  };
}
