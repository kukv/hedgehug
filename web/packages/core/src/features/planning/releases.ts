import type { Member } from "../../model/member";
import type { Release, ReleaseCard, ReleaseInput, ReleaseState } from "../../model/planning";
import type { WorkspaceDetail, WorkspaceSettings } from "../../model/workspace";
import type { PlanningPort } from "../../port/planning";
import { type Readable, readonly, writable } from "../../reactive/store";
import { readOnlyNote } from "../catalog/readonly";
import { formatDay } from "../dates/date";

// リリースの画面(05「リリース」)。カードは API が並べた順(ReleaseOrder)で受け取る。
// 保存はスプリントと同じ runSprintAction で行う(断りはダイアログの中、ほかの失敗はエラーのトースト。F1-18)。

const labels = {
  unreleased: "未リリース",
  released: "リリース済み",
  archived: "アーカイブ",
} as const;
const colors = { unreleased: "blue", released: "green", archived: "gray" } as const;
const order: ReleaseState[] = ["unreleased", "released", "archived"];

// stateLabel は、状態の表示。
export const stateLabel = (s: ReleaseState) => labels[s];

// stateColor は、状態のピルの色(ui の `--c-<色>-*` トークンの名前)。
export const stateColor = (s: ReleaseState) => colors[s];

// overdue は、未リリースで予定日が今日より前か。予定日が今日なら過ぎていない。予定日がなければ過ぎていない。
// today は、このマシンの時刻帯での今日(YYYY-MM-DD)。
export function overdue(r: Release, today: string): boolean {
  return r.state === "unreleased" && r.date !== "" && r.date < today;
}

// dateText は、カードの予定日。なければ「日付未定」。
export function dateText(r: Release, thisYear: number): string {
  return r.date === "" ? "日付未定" : formatDay(r.date, thisYear);
}

// progressText は、進捗のバーの横の文。対象は中止を除く Story・Task・Bug(API が数えている)。
export function progressText(c: ReleaseCard): string {
  const base = `${c.done} / ${c.target} 件完了`;
  return c.undone > 0 ? `${base} · 残り ${c.undone} 件` : base;
}

// progressRatio は、完了の割合(0〜1)。対象が 0 なら 0。
export function progressRatio(c: ReleaseCard): number {
  return c.target === 0 ? 0 : c.done / c.target;
}

// stateMenu は、メニューの「{状態}にする」。いまの状態を除く 2 つ。
export function stateMenu(r: Release): { to: ReleaseState; label: string }[] {
  return order.filter((s) => s !== r.state).map((to) => ({ to, label: `${labels[to]}にする` }));
}

// releaseConfirm は、リリース済みにするときの確認の本文。未完了(分類が未着手か進行中の Story・Task・Bug)が
// なければ null(確認せずに変える)。
export function releaseConfirm(c: ReleaseCard): string | null {
  if (c.undone === 0) return null;
  return `${c.release.name} には未完了のアイテムが ${c.undone} 件あります。このままリリース済みにしますか?`;
}

export interface ReleaseMenuEntry {
  // id は "edit"・"delete"、または変える先の状態。
  id: "edit" | "delete" | ReleaseState;
  label: string;
  disabled: boolean;
  hint?: string;
}

// releaseMenu は、カードのメニュー。「編集…」、状態 2 つ、「削除…」。閲覧のみの Store では、すべて無効にする(01)。
// saving は、そのリリースの状態を保存しているあいだ。状態の 2 つだけを無効にする(重ねて変えない)。
export function releaseMenu(r: Release, readOnly: boolean, saving = false): ReleaseMenuEntry[] {
  const entry = (id: ReleaseMenuEntry["id"], label: string, busy = false): ReleaseMenuEntry =>
    readOnly ? { id, label, disabled: true, hint: readOnlyNote } : { id, label, disabled: busy };
  return [
    entry("edit", "編集…"),
    ...stateMenu(r).map((m) => entry(m.to, m.label, saving)),
    entry("delete", "削除…"),
  ];
}

export const releaseCreated = (name: string) => `${name} を作成しました`;
export const releaseStateChanged = (name: string, to: ReleaseState) =>
  `${name} を${labels[to]}にしました`;
export const releaseDeleted = "リリースを削除しました";

// newReleaseForm は、作成ダイアログの初めの値(すべて空)。
export function newReleaseForm(): ReleaseInput {
  return { name: "", date: "", description: "" };
}

// releaseForm は、編集ダイアログの初めの値。
export function releaseForm(r: Release): ReleaseInput {
  return { name: r.name, date: r.date, description: r.description };
}

// deleteReleaseBody は、削除の確認ダイアログの本文(05「削除」)。items は入っているアイテムの数(削除済みを除く)、
// retentionDays は、その Store の削除済みの保持日数。
export function deleteReleaseBody(name: string, items: number, retentionDays: number): string {
  const inside = items > 0 ? `${items} 件のアイテムからリリースが解除されます。` : "";
  return `「${name}」を削除します。${inside}設定の「削除済み」から戻せます。${retentionDays} 日を過ぎると完全に削除されます。`;
}

export interface ReleasesState {
  storeId: string;
  workspaceId: string;
  cards: ReleaseCard[];
  // settings はステータスの名前、members は担当者の名前に使う。
  settings: WorkspaceSettings;
  members: Member[];
}

export interface ReleasesDeps {
  port: Pick<PlanningPort, "releaseCards">;
  workspaces: { get(storeId: string, workspaceId: string): Promise<WorkspaceDetail> };
  listMembers: (storeId: string) => Promise<Member[]>;
}

// Releases は、開いているワークスペースのリリースのカード。タブを開いたら読み、Store の出来事で読み直す。
export interface Releases {
  state: Readable<ReleasesState | null>;
  // 別のワークスペースなら、state を消してから読み込む。
  open(storeId: string, workspaceId: string): Promise<void>;
  // 開いていなければ何もしない。読めなかったときは、前の state のまま(断りは出さない。A2 の決定 29)。
  reload(): Promise<void>;
  // タブを離れたら、読み直しをやめる。
  close(): void;
}

export function createReleases(deps: ReleasesDeps): Releases {
  const state = writable<ReleasesState | null>(null);
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
        deps.port.releaseCards(storeId, workspaceId),
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
