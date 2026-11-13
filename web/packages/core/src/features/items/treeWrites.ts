import { NotFound, Refused } from "../../model/errors";
import type { Item } from "../../model/item";
import type { MoveCandidate } from "../../model/itemAction";
import type { WorkspaceSettings } from "../../model/workspace";
import type { Toasts } from "../toasts/toasts";
import { createKeyQueue } from "./keyQueue";
import { moveFailedMessage } from "./move";
import { type Dir, doneToggleTarget, reparentTarget } from "./treeMove";

// ツリーの書き込みのキーのうち、画面の部品を使わないもの(Space、Alt+↑↓、Alt+Shift+↑↓。04「ツリーのキー操作」)。
// 閲覧のみの Store・無効のときは、何もしない(トーストも出さない)。Alt+↑↓・Alt+Shift+↑↓ は順番待ち(A4b-21)に積み、
// 基準(兄弟・新しい親)は、実行の時点で読み直したあとの画面から作る。

// TreeScreen は、いまのツリーの画面。積んだ分を実行するたびに呼び直す(読み直しのあとの値を返す)。
export interface TreeScreen {
  storeId: string;
  readOnly: boolean;
  settings: WorkspaceSettings;
  // rows は、見えている行を上から順に並べたもの。
  rows: readonly Item[];
}

export interface TreeWritesDeps {
  screen(): TreeScreen | null;
  // reorder は、見えている兄弟を基準に 1 件だけ動かす(メニューの「上へ移動」「下へ移動」と同じ。itemMenuHost.reorder)。
  // 断りのトースト・読み直しまで行い、動かせたら true。見えている兄弟がない・閲覧のみの Store・断り・失敗は false。
  reorder(item: Item, rows: readonly Item[], dir: Dir): Promise<boolean>;
  moveCandidates(storeId: string, itemId: string): Promise<MoveCandidate[]>;
  // moveTo は、候補の下へ移す(「移動…」で選んだのと同じ。トースト・読み直しまで行う)。移せたら true。
  moveTo(item: Item, candidate: MoveCandidate): Promise<boolean>;
  // setStatus は、ステータスを変えて読み直す。断りはトーストに出す(projectScreen.setStatus)。
  setStatus(itemId: string, statusId: string): Promise<void>;
  // afterChange は、並べ替えのあとのツリー・詳細パネルの読み直し(itemRefresh.afterChange)。
  afterChange(): Promise<void>;
  // settled は、読み直した結果が画面の状態に届くのを待つ(app は svelte の tick)。
  settled(): Promise<void>;
  // expand は、行を開く(新しい親が閉じていれば開く)。
  expand(id: string): void;
  toasts: Pick<Toasts, "show">;
}

export interface TreeWrites {
  toggleDone(id: string): Promise<void>;
  // moveSibling は Alt+↑↓、changeParent は Alt+Shift+↑↓。順番待ちに積む。
  moveSibling(id: string, dir: Dir): void;
  changeParent(id: string, dir: Dir): void;
  // clear は、待っている分を捨てる(ほかのキー・画面を移ったとき)。
  clear(): void;
}

export function createTreeWrites(deps: TreeWritesDeps): TreeWrites {
  const queue = createKeyQueue();

  // writableRow は、書ける画面(閲覧のみでない)と、そこにある行 id を返す。
  function writableRow(id: string) {
    const s = deps.screen();
    const row = s?.rows.find((r) => r.id === id);
    return s && row && !s.readOnly ? { s, row } : null;
  }

  // failed は、候補を読めなかったとき、断られた(Refused)なら断りの文言を、すでに消えているなら読み直しを、
  // それ以外なら失敗のトーストを出す(移動のトーストと同じ)。
  async function failed(e: unknown) {
    if (e instanceof NotFound) await deps.afterChange();
    else deps.toasts.show(e instanceof Refused ? e.message : moveFailedMessage, { error: true });
  }

  async function siblingStep(id: string, dir: Dir): Promise<boolean> {
    const w = writableRow(id);
    if (!w || !(await deps.reorder(w.row, w.s.rows, dir))) return false;
    await deps.settled();
    return true;
  }

  async function parentStep(id: string, dir: Dir): Promise<boolean> {
    const w = writableRow(id);
    // Epic の候補はプロジェクトで、ツリーの行にない(プロジェクトの行は E)ので、何もしない。
    if (!w || w.row.kind === "Epic") return false;
    let candidates: MoveCandidate[];
    try {
      candidates = await deps.moveCandidates(w.s.storeId, id);
    } catch (e) {
      await failed(e);
      return false;
    }
    const now = writableRow(id);
    const target = now && reparentTarget(now.s.rows, id, dir, candidates);
    if (!now || !target) return false;
    // 新しい親が閉じていれば、移す前に開く(移した行が見えなくなって、フォーカスが次の行へ移るのを避ける)。
    deps.expand(target.id);
    if (!(await deps.moveTo(now.row, target))) return false;
    await deps.settled();
    return true;
  }

  return {
    async toggleDone(id) {
      const w = writableRow(id);
      const status = w && doneToggleTarget(w.s.settings, w.row.status);
      if (w && status && status !== w.row.status) await deps.setStatus(id, status);
    },
    moveSibling: (id, dir) => queue.push(() => siblingStep(id, dir)),
    changeParent: (id, dir) => queue.push(() => parentStep(id, dir)),
    clear: () => queue.clear(),
  };
}
