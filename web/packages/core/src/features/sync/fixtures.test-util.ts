import type { SyncPort } from "../../port/sync";

// fakeSyncPort は、呼ばれた操作を calls に「名前 引数」で残す偽の SyncPort。over で応答を差し替える。
export function fakeSyncPort(over: Partial<SyncPort> = {}): SyncPort & { calls: string[] } {
  const calls: string[] = [];
  const port: SyncPort = {
    status: async () => ({ auto: true, stores: [] }),
    syncStore: async () => ({ result: "synced", message: "", mirrorWarning: "" }),
    syncAll: async () => ({ result: "synced", lines: [] }),
    hold: async () => {},
    network: async () => {},
    watch: () => ({ done: Promise.resolve(), stop: () => {} }),
    ...over,
  };
  return {
    calls,
    status: () => {
      calls.push("status");
      return port.status();
    },
    syncStore: (id) => {
      calls.push(`syncStore ${id}`);
      return port.syncStore(id);
    },
    syncAll: () => {
      calls.push("syncAll");
      return port.syncAll();
    },
    hold: (tab, hold) => {
      calls.push(`hold ${tab} ${hold}`);
      return port.hold(tab, hold);
    },
    network: (tab, online) => {
      calls.push(`network ${tab} ${online}`);
      return port.network(tab, online);
    },
    watch: (on) => {
      calls.push("watch");
      return port.watch(on);
    },
  };
}
