import type { Report, ReportActive, ReportSelection } from "../../model/report";
import type { ReportsPort } from "../../port/reports";
import { type Readable, readonly, writable } from "../../reactive/store";

// レポートの画面の状態(05「レポート」)。タブを開いたら読み、Store の出来事で読み直す。

type Period = { from: string; to: string };
type Target = { storeId: string; workspaceId: string };

export interface ReportsState {
  storeId: string;
  workspaceId: string;
  report: Report;
}

export interface Reports {
  state: Readable<ReportsState | null>;
  // 別のワークスペースなら、state を消してから読み込む。
  open(storeId: string, workspaceId: string): Promise<void>;
  // 開いていなければ何もしない。読めなかったときは、前の state のまま(断りは出さない。A2 の決定 29)。
  reload(): Promise<void>;
  // 画面を離れたら、読み直しをやめ、期間を初めの期間に戻す。
  close(): void;
  // 変えた項目だけを保存してから読み直す。保存に失敗したら投げる(呼び出し側が「保存できませんでした」を出す)。
  select(patch: Partial<ReportSelection>): Promise<void>;
  // 累積フロー図の期間。null は初めの期間(直近 21 日)。読めなかった期間は覚えない。
  setPeriod(p: Period | null): Promise<void>;
}

const same = (a: Target | null, storeId: string, workspaceId: string) =>
  a?.storeId === storeId && a.workspaceId === workspaceId;

export function createReports(port: ReportsPort): Reports {
  const state = writable<ReportsState | null>(null);
  let current: Target | null = null;
  // 期間は保存しない。close したら初めの期間(null)に戻す。
  let period: Period | null = null;
  // 読み直しが重なったときは、最後に始めたものの結果だけを使う。
  let latest = 0;

  async function load(next: Period | null) {
    if (!current) return;
    const { storeId, workspaceId } = current;
    latest += 1;
    const mine = latest;
    try {
      const report = await port.report(storeId, workspaceId, next);
      if (mine !== latest) return;
      period = next;
      state.set({ storeId, workspaceId, report });
    } catch {
      // 読めなければ前のまま(期間も変えない)。
    }
  }

  const reload = () => load(period);

  return {
    state: readonly(state),
    async open(storeId, workspaceId) {
      if (!same(current, storeId, workspaceId)) {
        state.set(null);
        period = null;
      }
      current = { storeId, workspaceId };
      await reload();
    },
    reload,
    close() {
      current = null;
      period = null;
      latest += 1;
      state.set(null);
    },
    async select(patch) {
      if (!current) return;
      await port.select(current.storeId, current.workspaceId, patch);
      await reload();
    },
    setPeriod: (p) => load(p),
  };
}

export interface ActiveReportsState {
  storeId: string;
  workspaceId: string;
  active: ReportActive;
}

// ActiveReports は、実行中のスプリントの数字(スプリントの画面のカードと概要が使う)。
export interface ActiveReports {
  state: Readable<ActiveReportsState | null>;
  open(storeId: string, workspaceId: string): Promise<void>;
  reload(): Promise<void>;
  close(): void;
}

export function createActiveReports(port: ReportsPort): ActiveReports {
  const state = writable<ActiveReportsState | null>(null);
  let current: Target | null = null;
  let latest = 0;

  async function reload() {
    if (!current) return;
    const { storeId, workspaceId } = current;
    latest += 1;
    const mine = latest;
    try {
      const active = await port.active(storeId, workspaceId);
      if (mine === latest) state.set({ storeId, workspaceId, active });
    } catch {
      // 読めなければ前のまま。
    }
  }

  return {
    state: readonly(state),
    async open(storeId, workspaceId) {
      if (!same(current, storeId, workspaceId)) state.set(null);
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
