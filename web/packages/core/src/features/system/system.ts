import type { SystemPort } from "../../port/system";
import { type Readable, readonly, writable } from "../../reactive/store";

// サーバーの版。読めなかったときは null のまま。
export interface SystemInfoState {
  version: Readable<string | null>;
  load(): Promise<boolean>;
}

export function createSystemInfo(port: SystemPort): SystemInfoState {
  const version = writable<string | null>(null);
  return {
    version: readonly(version),
    async load() {
      try {
        version.set((await port.getSystem()).version);
        return true;
      } catch {
        return false;
      }
    },
  };
}
