import type { PlanningOverview } from "../../model/planning";
import type { PlanningPort } from "../../port/planning";
import { type Readable, readonly, writable } from "../../reactive/store";
import { formatDay } from "../dates/date";

// activeLabel は、ワークスペースの見出しの補足(05「実行中: {名前}({開始日}〜{終了日})」)。
// 実行中のスプリントがなければ null。thisYear は、このマシンの時刻帯での今年。
export function activeLabel(o: PlanningOverview, thisYear: number): string | null {
  const a = o.active;
  if (!a) return null;
  return `実行中: ${a.name}(${formatDay(a.start, thisYear)}〜${formatDay(a.end, thisYear)})`;
}

export interface PlanningState {
  storeId: string;
  workspaceId: string;
  overview: PlanningOverview;
}

// Planning は、開いているワークスペースのスプリントとリリースの概要。Store の出来事で読み直す。
export interface Planning {
  state: Readable<PlanningState | null>;
  // 別のワークスペースなら、state を消してから読み込む(古いワークスペースを新しいアドレスの下に出さない)。
  open(storeId: string, workspaceId: string): Promise<void>;
  // 開いていなければ何もしない。読めなかったときは、前の state のまま(断りは出さない。A2 の決定 29)。
  reload(): Promise<void>;
  // 画面を閉じたら、読み直しをやめる。
  close(): void;
}

export function createPlanning(port: PlanningPort): Planning {
  const state = writable<PlanningState | null>(null);
  let current: { storeId: string; workspaceId: string } | null = null;
  // 読み直しが重なったときは、最後に始めたものの結果だけを使う。
  let latest = 0;

  async function reload() {
    if (!current) return;
    const { storeId, workspaceId } = current;
    latest += 1;
    const mine = latest;
    try {
      const overview = await port.overview(storeId, workspaceId);
      if (mine === latest) state.set({ storeId, workspaceId, overview });
    } catch {
      // 見出しの補足なので、読めなければ前のまま(別のワークスペースなら出さない)。
    }
  }

  return {
    state: readonly(state),
    async open(storeId, workspaceId) {
      if (current?.storeId !== storeId || current.workspaceId !== workspaceId) state.set(null);
      current = { storeId, workspaceId };
      await reload();
    },
    reload,
    close() {
      current = null;
      latest += 1;
      state.set(null);
    },
  };
}
