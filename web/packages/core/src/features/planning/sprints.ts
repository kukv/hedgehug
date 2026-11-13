import type { Member } from "../../model/member";
import type { PlanningOverview, Sprint, SprintCard, SprintState } from "../../model/planning";
import type { WorkspaceDetail, WorkspaceSettings } from "../../model/workspace";
import type { PlanningPort } from "../../port/planning";
import { type Readable, readonly, writable } from "../../reactive/store";
import { readOnlyNote } from "../catalog/readonly";
import { formatDay } from "../dates/date";

// スプリントの画面(05「スプリントの画面」)。カードは API が並べた順(ScreenOrder)で受け取る。

const labels = { active: "実行中", planned: "計画中", closed: "完了" } as const;
const order: SprintState[] = ["active", "planned", "closed"];

export interface SprintSection {
  state: SprintState;
  label: (typeof labels)[SprintState];
  cards: SprintCard[];
}

// sections は、実行中・計画中・完了の順の見出しと、その下のカード。カードのない状態は出さない。
export function sections(cards: SprintCard[]): SprintSection[] {
  return order
    .map((state) => ({
      state,
      label: labels[state],
      cards: cards.filter((c) => c.sprint.state === state),
    }))
    .filter((s) => s.cards.length > 0);
}

const dayNumber = (ymd: string) => {
  const [y, m, d] = ymd.split("-").map(Number);
  return Date.UTC(y ?? 0, (m ?? 1) - 1, d ?? 1) / 86_400_000;
};

// pill は、実行中のカードの見出しに添えるピル。終了日の当日は「残り 1 日」。実行中でなければ null。
// today は、このマシンの時刻帯での今日(YYYY-MM-DD)。
export function pill(s: Sprint, today: string): string | null {
  if (s.state !== "active") return null;
  const left = dayNumber(s.end) - dayNumber(today) + 1;
  return left >= 1 ? `残り ${left} 日` : "期限を過ぎています";
}

// ポイントは、詳細パネルのポイントと同じ書き方(数をそのまま)。
const pt = (n: number) => String(n);

// progressText は、進捗のバーの横の文。中止は API が除いて数えている。完了したスプリントには確定値を続ける。
export function progressText(c: SprintCard): string {
  const base = `${c.done.count} / ${c.total.count} 件 · ${pt(c.done.points)} / ${pt(c.total.points)} pt`;
  const f = c.sprint.confirmed;
  if (c.sprint.state !== "closed" || !f) return base;
  return `${base} · 確定 ${pt(f.completed.points)}/${pt(f.committed.points)} pt`;
}

// progressRatio は、完了ポイントの割合(0〜1)。全体が 0 なら 0。
export function progressRatio(c: SprintCard): number {
  return c.total.points === 0 ? 0 : c.done.points / c.total.points;
}

// periodText は、カードの期間。開始日か終了日がなければ「期間未定」。
export function periodText(s: Sprint, thisYear: number): string {
  if (s.start === "" || s.end === "") return "期間未定";
  return `${formatDay(s.start, thisYear)} 〜 ${formatDay(s.end, thisYear)}`;
}

// startDisabledReason は、「開始」を押せない理由(実行中のスプリントがあるとき)。押せるなら null。
export function startDisabledReason(o: PlanningOverview): string | null {
  return o.active ? "実行中のスプリントを先に完了してください" : null;
}

export interface SprintMenuEntry {
  id: "edit" | "delete";
  label: string;
  disabled: boolean;
  hint?: string;
}

const entry = (id: SprintMenuEntry["id"], label: string, why: string | null): SprintMenuEntry =>
  why ? { id, label, disabled: true, hint: why } : { id, label, disabled: false };

// sprintMenu は、カードのメニュー(F1-19 で「編集…」「削除…」だけ)。閲覧のみの Store では、どちらも無効にする(01)。
export function sprintMenu(s: Sprint, readOnly: boolean): SprintMenuEntry[] {
  const ro = readOnly ? readOnlyNote : null;
  return [
    entry("edit", "編集…", ro),
    entry("delete", "削除…", ro ?? (s.state === "active" ? "実行中は削除できません" : null)),
  ];
}

export interface SprintsState {
  storeId: string;
  workspaceId: string;
  cards: SprintCard[];
  // settings はステータスの名前、members は担当者の名前に使う。
  settings: WorkspaceSettings;
  members: Member[];
}

export interface SprintsDeps {
  port: Pick<PlanningPort, "sprintCards">;
  workspaces: { get(storeId: string, workspaceId: string): Promise<WorkspaceDetail> };
  listMembers: (storeId: string) => Promise<Member[]>;
}

// Sprints は、開いているワークスペースのスプリントのカード。タブを開いたら読み、Store の出来事で読み直す。
export interface Sprints {
  state: Readable<SprintsState | null>;
  // 別のワークスペースなら、state を消してから読み込む。
  open(storeId: string, workspaceId: string): Promise<void>;
  // 開いていなければ何もしない。読めなかったときは、前の state のまま(断りは出さない。A2 の決定 29)。
  reload(): Promise<void>;
  // タブを離れたら、読み直しをやめる。
  close(): void;
}

export function createSprints(deps: SprintsDeps): Sprints {
  const state = writable<SprintsState | null>(null);
  let current: { storeId: string; workspaceId: string } | null = null;
  // 読み直しが重なったときは、最後に始めたものの結果だけを使う。
  let latest = 0;

  async function reload() {
    if (!current) return;
    const { storeId, workspaceId } = current;
    latest += 1;
    const mine = latest;
    try {
      const [cards, ws, members] = await Promise.all([
        deps.port.sprintCards(storeId, workspaceId),
        deps.workspaces.get(storeId, workspaceId),
        deps.listMembers(storeId),
      ]);
      if (mine === latest)
        state.set({ storeId, workspaceId, cards, settings: ws.settings, members });
    } catch {
      // 読めなければ前のまま(別のワークスペースなら出さない)。
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
