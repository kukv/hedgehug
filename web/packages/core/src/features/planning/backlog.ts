import { Refused } from "../../model/errors";
import type { Item } from "../../model/item";
import type { Member } from "../../model/member";
import type { BacklogFrame, BacklogRow } from "../../model/planning";
import type { WorkspaceSettings } from "../../model/workspace";
import type { ItemsPort } from "../../port/items";
import type { PlanningPort } from "../../port/planning";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Toasts } from "../toasts/toasts";
import { type Drop, goneText } from "./backlogDrop";
import { sprintSaveFailed } from "./sprintDialogs";
import { pt, type SprintsDeps } from "./sprints";

// バックログの画面の枠と、行の操作の保存(05「バックログ」、F2a-8・F2a-9・F2a-10)。

export interface BacklogState {
  storeId: string;
  workspaceId: string;
  // frames は、API が並べた枠(スプリントの枠、最後にバックログの枠)。
  frames: BacklogFrame[];
  // settings はステータスの名前と分類、members は担当者の名前に使う。
  settings: WorkspaceSettings;
  members: Member[];
}

export interface BacklogDeps {
  port: Pick<PlanningPort, "backlog" | "placeItem" | "setItemSprint">;
  items: Pick<ItemsPort, "setProperty">;
  workspaces: SprintsDeps["workspaces"];
  listMembers: SprintsDeps["listMembers"];
  toasts: Toasts;
  openDetail(storeId: string, itemId: string): void;
}

// Backlog は、開いているワークスペースのバックログの画面。タブを開いたら読み、保存のあとと Store の出来事で読み直す。
// 保存の断りはその文を、ほかの失敗は「保存できませんでした」をエラーのトーストに出し、どちらも読み直す(F2a-9)。
export interface Backlog {
  state: Readable<BacklogState | null>;
  // 別のワークスペースなら、state を消してから読み込む。
  open(storeId: string, workspaceId: string): Promise<void>;
  // 開いていなければ何もしない。読めなかったときは、前の state のまま(断りは出さない)。
  reload(): Promise<void>;
  // タブを離れたら、読み直しをやめ、詳細パネルを開いた行を忘れる。
  close(): void;
  setStatus(row: BacklogRow, statusId: string): Promise<void>;
  // setPoints は、ポイントをその場の入力 text(空は値の削除)で保存する。false なら、入力を保存済みの値に戻す。
  setPoints(row: BacklogRow, text: string): Promise<boolean>;
  // setSprint は、スプリントのセレクトの値(空はバックログ)で保存する。同じ値なら送らない。
  setSprint(row: BacklogRow, sprintId: string): Promise<void>;
  place(row: BacklogRow, drop: Drop): Promise<void>;
  // openDetail は、行の詳細パネルを開き、開いたアイテムを覚える(noticeDetail が使う)。
  openDetail(row: BacklogRow): void;
  // noticeDetail は、詳細パネルの保存の応答 item を受け取る。この画面から開いたアイテムが、開いている枠の
  // どのスプリントにも入っておらず完了・中止なら、消えるときのトーストを出す(F2a-10 の(2))。
  noticeDetail(storeId: string, item: Item): void;
}

// goneCategory は、ステータスの分類が完了・中止ならその分類。ほか(ないステータスは未着手)は null。
function goneCategory(settings: WorkspaceSettings, statusId: string) {
  const c = settings.statuses.find((s) => s.id === statusId)?.category;
  return c === "done" || c === "canceled" ? c : null;
}

// inSprint は、行がスプリントの枠にあるか。
const inSprint = (s: BacklogState, row: BacklogRow): boolean =>
  s.frames.some((f) => f.sprint !== null && f.rows.some((r) => r.id === row.id));

export function createBacklog(deps: BacklogDeps): Backlog {
  const state = writable<BacklogState | null>(null);
  let current: { storeId: string; workspaceId: string } | null = null;
  // 読み直しが重なったときは、最後に始めたものの結果だけを使う。
  let latest = 0;
  // opened は、この画面の行から開いた詳細パネルのアイテム。
  let opened: { id: string; display: string } | null = null;

  async function reload() {
    if (!current) return;
    const { storeId, workspaceId } = current;
    latest += 1;
    const mine = latest;
    try {
      const [frames, ws, members] = await Promise.all([
        deps.port.backlog(storeId, workspaceId),
        deps.workspaces.get(storeId, workspaceId),
        deps.listMembers(storeId),
      ]);
      if (mine === latest)
        state.set({ storeId, workspaceId, frames, settings: ws.settings, members });
    } catch {
      // 読めなければ前のまま(別のワークスペースなら出さない)。
    }
  }

  // gone は、ステータスの分類が完了・中止なら、消えるときのトーストを出す。
  function gone(s: BacklogState, display: string, statusId: string) {
    const c = goneCategory(s.settings, statusId);
    if (c) deps.toasts.show(goneText(display, c));
  }

  // save は、開いている画面で write を保存し、読み直す。保存できたら true。
  async function save(write: (s: BacklogState) => Promise<void>): Promise<boolean> {
    const s = get(state);
    if (!s) return false;
    let ok = true;
    try {
      await write(s);
    } catch (e) {
      deps.toasts.show(e instanceof Refused ? e.message : sprintSaveFailed, { error: true });
      ok = false;
    }
    await reload();
    return ok;
  }

  return {
    state: readonly(state),
    async open(storeId, workspaceId) {
      if (current?.storeId !== storeId || current.workspaceId !== workspaceId) {
        state.set(null);
        opened = null;
      }
      current = { storeId, workspaceId };
      await reload();
    },
    reload,
    close() {
      current = null;
      latest += 1;
      opened = null;
      state.set(null);
    },
    async setStatus(row, statusId) {
      await save(async (s) => {
        const saved = await deps.items.setProperty(s.storeId, row.id, "status", statusId);
        if (!inSprint(s, row)) gone(s, row.display, saved.status);
      });
    },
    async setPoints(row, text) {
      if (text === (row.points === null ? "" : pt(row.points))) return true;
      return save(async (s) => {
        await deps.items.setProperty(s.storeId, row.id, "points", text);
      });
    },
    async setSprint(row, sprintId) {
      const now =
        get(state)?.frames.find((f) => f.rows.some((r) => r.id === row.id))?.sprint?.id ?? "";
      if (sprintId === now) return;
      await save(async (s) => {
        await deps.port.setItemSprint(s.storeId, row.id, sprintId || null);
        if (sprintId === "") gone(s, row.display, row.status);
      });
    },
    async place(row, drop) {
      await save(async (s) => {
        await deps.port.placeItem(s.storeId, row.id, drop.sprint, drop.anchor, drop.after);
        if (drop.sprint === null && inSprint(s, row)) gone(s, row.display, row.status);
      });
    },
    openDetail(row) {
      const s = get(state);
      if (!s) return;
      opened = { id: row.id, display: row.display };
      deps.openDetail(s.storeId, row.id);
    },
    noticeDetail(storeId, item) {
      const s = get(state);
      if (!s || s.storeId !== storeId || opened?.id !== item.id) return;
      if (s.frames.some((f) => f.sprint !== null && f.sprint.id === item.sprint)) return;
      gone(s, opened.display, item.status);
    },
  };
}
