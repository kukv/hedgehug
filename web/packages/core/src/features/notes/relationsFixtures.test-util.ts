import { NotFound, Refused } from "../../model/errors";
import type { RelationCandidate, RelationLabel, RelationRow } from "../../model/relation";
import { createToasts } from "../toasts/toasts";
import { createRelations, type RelationsDeps } from "./relations";

export const row = (label: RelationLabel, id: string, display: string): RelationRow => ({
  label,
  id,
  display,
  title: `${display} のタイトル`,
  workspaceName: "",
  statusName: "Todo",
  statusColor: "gray",
});
export const cand = (id: string, display: string, disabledReason = ""): RelationCandidate => ({
  id,
  display,
  title: `${display} のタイトル`,
  workspaceName: "",
  disabledReason,
});
export const first = <T>(xs: T[]): T => {
  const x = xs[0];
  if (x === undefined) throw new Error("empty");
  return x;
};

export function setup(initial: RelationRow[] = [row("blocks", "I2", "HH-2")]) {
  const calls: string[] = [];
  let rows = initial;
  // answers は、`${表記}|${検索語}` ごとの候補。hold に入れた検索語の問い合わせは、waiting の関数を呼ぶまで答えない。
  const answers = new Map<string, RelationCandidate[]>();
  const hold = new Set<string>();
  const waiting = new Map<string, (cs: RelationCandidate[]) => void>();
  let refusal = "";
  let missing = false;
  // failure は、次の追加・解除を、この失敗にする。releaseAdd は、holdAdd した追加を終わらせる。
  let failure: Error | null = null;
  let releaseAdd: (() => void) | null = null;
  let holdAdd = false;
  const notes: RelationsDeps["notes"] = {
    async listRelations(storeId, itemId) {
      calls.push(`list ${storeId}/${itemId}`);
      if (missing) throw new NotFound("not found");
      return rows;
    },
    relationCandidates(_s, _i, label, query) {
      calls.push(`candidates ${label} ${query}`);
      if (hold.has(query)) return new Promise((resolve) => waiting.set(query, resolve));
      return Promise.resolve(answers.get(`${label}|${query}`) ?? []);
    },
    async addRelation(_s, _i, label, other) {
      calls.push(`add ${label} ${other}`);
      if (holdAdd) await new Promise<void>((resolve) => (releaseAdd = resolve));
      if (failure) throw failure;
      if (refusal) throw new Refused(refusal);
      rows = [...rows, row(label, other, other)];
    },
    async removeRelation(_s, _i, label, other) {
      calls.push(`remove ${label} ${other}`);
      if (failure) throw failure;
      rows = rows.filter((r) => !(r.label === label && r.id === other));
    },
  };
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  let changed = 0;
  const relations = createRelations({ notes, toasts, onChanged: () => void changed++ });
  return {
    relations,
    toasts,
    calls,
    answers,
    hold,
    waiting,
    changed: () => changed,
    refuse: (m: string) => {
      refusal = m;
    },
    vanish: () => {
      missing = true;
    },
    failWith: (e: Error) => {
      failure = e;
    },
    holdAdds: () => {
      holdAdd = true;
    },
    finishAdd: () => releaseAdd?.(),
  };
}
