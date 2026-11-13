import { NotFound, Refused } from "../../model/errors";
import type { RelationCandidate, RelationLabel, RelationRow } from "../../model/relation";
import { relationLabels } from "../../model/relation";
import type { NotesPort } from "../../port/notes";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Toasts } from "../toasts/toasts";

// 詳細パネルの「関連」(02「関連」)の状態: 一覧、解除、追加の行(表記・相手の検索・選んだ相手)。

// 断りでも「ない」でもない失敗(通信の失敗など)のときのエラーのトースト(02「関連」)。
export const relationFailedMessage = "関連を保存できませんでした";

export interface RelationGroup {
  label: RelationLabel;
  rows: RelationRow[];
}

// groupRelations は、一覧を表記ごとの見出しにまとめる。表記の順で、関連のない表記は出さない。
export function groupRelations(rows: RelationRow[]): RelationGroup[] {
  return relationLabels
    .map((label) => ({ label, rows: rows.filter((r) => r.label === label) }))
    .filter((g) => g.rows.length > 0);
}

export interface RelationsState {
  // loaded は、一覧を読み終えたか(読むまでは「関連はありません」を出さない)。
  loaded: boolean;
  rows: RelationRow[];
  groups: RelationGroup[];
  // 追加の行: 表記のセレクト、相手の入力欄、候補、選んだ相手。
  label: RelationLabel;
  query: string;
  candidates: RelationCandidate[];
  selected: RelationCandidate | null;
  // canAdd は、「追加」が押せるか(選べる相手を選んでいる)。
  canAdd: boolean;
  // busy は、追加している間(二重に足さない)。
  busy: boolean;
}

export interface Relations {
  state: Readable<RelationsState>;
  // load は、アイテムの関連を読む。同じアイテムの読み直しでは、追加の行を残す。
  load(storeId: string, itemId: string): Promise<void>;
  // reload は、画面への反映(06)。一覧だけ読み直し、追加の行は書き換えない。開いていなければ何もしない。
  reload(): Promise<void>;
  setLabel(label: RelationLabel): Promise<void>;
  setQuery(query: string): Promise<void>;
  // choose は、候補を選ぶ。選べない候補は無視する。
  choose(candidate: RelationCandidate): void;
  // add は、選んだ相手を、いまの表記で足す。足せたら true。
  add(): Promise<boolean>;
  remove(row: RelationRow): Promise<void>;
  reset(): void;
}

export interface RelationsDeps {
  notes: Pick<NotesPort, "listRelations" | "relationCandidates" | "addRelation" | "removeRelation">;
  toasts: Toasts;
  // onChanged は、関連を足す・外したあと(ブロック中が変わるので、開いている詳細パネルとツリーを読み直す)。
  onChanged(): void;
}

const blank: RelationsState = {
  loaded: false,
  rows: [],
  groups: [],
  label: "blocks",
  query: "",
  candidates: [],
  selected: null,
  canAdd: false,
  busy: false,
};

export function createRelations(deps: RelationsDeps): Relations {
  const state = writable<RelationsState>(blank);
  let target: { storeId: string; itemId: string } | null = null;
  // loads・searches は、古い答えを捨てるための通し番号。
  let loads = 0;
  let searches = 0;

  const patch = (p: Partial<RelationsState>) =>
    state.update((s) => {
      const next = { ...s, ...p };
      next.groups = groupRelations(next.rows);
      next.canAdd = next.selected !== null && next.selected.disabledReason === "";
      return next;
    });

  async function loadRows() {
    if (!target) return;
    const t = target;
    loads += 1;
    const mine = loads;
    try {
      const rows = await deps.notes.listRelations(t.storeId, t.itemId);
      if (mine === loads) patch({ rows, loaded: true });
    } catch (e) {
      // アイテムがなくなっていたら空にする。ほかの失敗は出さない(画面への反映は失敗を出さない。A2 の決定 29)。
      if (e instanceof NotFound && mine === loads) patch({ rows: [], loaded: true });
    }
  }

  // runSearch は、候補を問い合わせる。古くなった答えと失敗は null(検索の失敗は、次の入力でやり直せる)。
  async function runSearch(query: string): Promise<RelationCandidate[] | null> {
    if (!target) return null;
    searches += 1;
    const mine = searches;
    if (query.trim() === "") return [];
    try {
      const list = await deps.notes.relationCandidates(
        target.storeId,
        target.itemId,
        get(state).label,
        query,
      );
      return mine === searches ? list : null;
    } catch (e) {
      if (e instanceof NotFound && mine === searches) return [];
      return null;
    }
  }

  // failed は、書き込みの失敗の扱い。ほかで消えていたら黙って読み直し、断りは文言を、それ以外は失敗を出す。
  function failed(e: unknown) {
    if (e instanceof NotFound) return;
    deps.toasts.show(e instanceof Refused ? e.message : relationFailedMessage, { error: true });
  }

  return {
    state: readonly(state),
    async load(storeId, itemId) {
      const same = target?.storeId === storeId && target.itemId === itemId;
      // 同じアイテムなら target を作り直さない(書き込みの途中で、同じアイテムかを target の同一性で見分けるため)。
      if (!same) {
        target = { storeId, itemId };
        searches += 1;
        state.set(blank);
      }
      await loadRows();
    },
    reload: loadRows,
    async setLabel(label) {
      patch({ label });
      const picked = get(state).selected;
      const list = await runSearch(picked ? picked.display : get(state).query);
      if (!list) return;
      if (!picked) {
        patch({ candidates: list });
        return;
      }
      const again = list.find((c) => c.id === picked.id);
      if (again && again.disabledReason === "") patch({ selected: again });
      else patch({ selected: null, query: picked.display, candidates: list });
    },
    async setQuery(query) {
      patch({ query, selected: null });
      const list = await runSearch(query);
      if (list) patch({ candidates: list });
    },
    choose(candidate) {
      if (candidate.disabledReason !== "") return;
      searches += 1;
      patch({
        selected: candidate,
        query: `${candidate.display} ${candidate.title}`,
        candidates: [],
      });
    },
    async add() {
      const { selected, label, busy } = get(state);
      if (!target || !selected || selected.disabledReason !== "" || busy) return false;
      const t = target;
      patch({ busy: true });
      try {
        await deps.notes.addRelation(t.storeId, t.itemId, label, selected.id);
      } catch (e) {
        failed(e);
        // 待っているあいだに別のアイテムへ移っていたら、そちらの状態は触らない。
        if (target === t) patch({ busy: false });
        await loadRows();
        return false;
      }
      if (target === t) {
        searches += 1;
        patch({ query: "", candidates: [], selected: null, busy: false });
      }
      await loadRows();
      deps.onChanged();
      return true;
    },
    async remove(row) {
      if (!target) return;
      try {
        await deps.notes.removeRelation(target.storeId, target.itemId, row.label, row.id);
      } catch (e) {
        failed(e);
        await loadRows();
        return;
      }
      await loadRows();
      deps.onChanged();
    },
    reset() {
      target = null;
      loads += 1;
      searches += 1;
      state.set(blank);
    },
  };
}
