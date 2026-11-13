import { describe, expect, it } from "vitest";
import { NotFound, Refused } from "../../model/errors";
import type { RelationCandidate, RelationLabel, RelationRow } from "../../model/relation";
import { get } from "../../reactive/store";
import { flush } from "../items/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import {
  createRelations,
  groupRelations,
  type RelationsDeps,
  relationFailedMessage,
} from "./relations";

const row = (label: RelationLabel, id: string, display: string): RelationRow => ({
  label,
  id,
  display,
  title: `${display} のタイトル`,
  workspaceName: "",
  statusName: "Todo",
  statusColor: "gray",
});
const cand = (id: string, display: string, disabledReason = ""): RelationCandidate => ({
  id,
  display,
  title: `${display} のタイトル`,
  workspaceName: "",
  disabledReason,
});
const first = <T>(xs: T[]): T => {
  const x = xs[0];
  if (x === undefined) throw new Error("empty");
  return x;
};

function setup(initial: RelationRow[] = [row("blocks", "I2", "HH-2")]) {
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

describe("groupRelations", () => {
  it("表記の順に見出しをまとめ、関連のない表記は出さない", () => {
    const groups = groupRelations([
      row("clones", "I5", "HH-5"),
      row("blocked by", "I1", "HH-1"),
      row("blocks", "I2", "HH-2"),
      row("blocks", "I3", "HH-3"),
    ]);
    expect(groups.map((g) => [g.label, g.rows.map((r) => r.display)])).toEqual([
      ["blocks", ["HH-2", "HH-3"]],
      ["blocked by", ["HH-1"]],
      ["clones", ["HH-5"]],
    ]);
  });
});

describe("createRelations", () => {
  it("読む前は loaded が false、読んだあとは一覧と見出しを持つ", async () => {
    const { relations } = setup();
    expect(get(relations.state)).toMatchObject({ loaded: false, rows: [], groups: [] });
    await relations.load("S1", "I1");
    const s = get(relations.state);
    expect(s.loaded).toBe(true);
    expect(s.groups.map((g) => g.label)).toEqual(["blocks"]);
    expect(s.label).toBe("blocks");
  });

  it("関連がなければ、読んだあとも一覧は空", async () => {
    const { relations } = setup([]);
    await relations.load("S1", "I1");
    expect(get(relations.state)).toMatchObject({ loaded: true, rows: [], groups: [] });
  });

  it("アイテムがなくなっていたら、空の一覧にする", async () => {
    const { relations, vanish } = setup();
    vanish();
    await relations.load("S1", "I1");
    expect(get(relations.state)).toMatchObject({ loaded: true, rows: [] });
  });

  it("検索して選ぶと「追加」が押せる。選べない候補は選べない", async () => {
    const { relations, answers } = setup();
    answers.set("blocks|hh", [
      cand("I3", "HH-3"),
      cand("I4", "HH-4", "すでに blocks の関連があります"),
    ]);
    await relations.load("S1", "I1");
    await relations.setQuery("hh");
    expect(get(relations.state).candidates.map((c) => c.id)).toEqual(["I3", "I4"]);
    relations.choose(get(relations.state).candidates[1] as RelationCandidate);
    expect(get(relations.state)).toMatchObject({ selected: null, canAdd: false });
    relations.choose(first(get(relations.state).candidates));
    expect(get(relations.state)).toMatchObject({
      selected: { id: "I3" },
      query: "HH-3 HH-3 のタイトル",
      candidates: [],
      canAdd: true,
    });
  });

  it("入力し直すと、選んだ相手を外す。空の入力は問い合わせない", async () => {
    const { relations, answers, calls } = setup();
    answers.set("blocks|hh", [cand("I3", "HH-3")]);
    await relations.load("S1", "I1");
    await relations.setQuery("hh");
    relations.choose(first(get(relations.state).candidates));
    await relations.setQuery("h");
    expect(get(relations.state)).toMatchObject({ selected: null, canAdd: false });
    await relations.setQuery("  ");
    expect(get(relations.state).candidates).toEqual([]);
    expect(calls.filter((c) => c.startsWith("candidates")).length).toBe(2);
  });

  it("追加すると保存し、一覧を読み直し、入力を空にして、画面に知らせる", async () => {
    const { relations, answers, calls, changed } = setup();
    answers.set("blocks|hh", [cand("I3", "HH-3")]);
    await relations.load("S1", "I1");
    await relations.setQuery("hh");
    relations.choose(first(get(relations.state).candidates));
    expect(await relations.add()).toBe(true);
    expect(calls).toContain("add blocks I3");
    expect(calls.filter((c) => c.startsWith("list")).length).toBe(2);
    expect(get(relations.state)).toMatchObject({
      query: "",
      candidates: [],
      selected: null,
      label: "blocks",
    });
    expect(get(relations.state).rows.map((r) => r.id)).toEqual(["I2", "I3"]);
    expect(changed()).toBe(1);
  });

  it("相手を選んでいなければ追加しない", async () => {
    const { relations, calls, changed } = setup();
    await relations.load("S1", "I1");
    expect(await relations.add()).toBe(false);
    expect(calls.some((c) => c.startsWith("add"))).toBe(false);
    expect(changed()).toBe(0);
  });

  it("断られたら、エラーのトーストを出し、選んだ相手は残す", async () => {
    const { relations, answers, toasts, refuse, changed } = setup();
    answers.set("blocks|hh", [cand("I3", "HH-3")]);
    await relations.load("S1", "I1");
    await relations.setQuery("hh");
    relations.choose(first(get(relations.state).candidates));
    refuse("循環になります");
    expect(await relations.add()).toBe(false);
    expect(get(toasts.list)[0]).toMatchObject({ text: "循環になります", error: true });
    expect(get(relations.state).selected?.id).toBe("I3");
    expect(changed()).toBe(0);
  });

  it("表記を変えると、選んだ相手を新しい表記で調べ直す", async () => {
    const { relations, answers, calls } = setup();
    answers.set("blocks|hh", [cand("I3", "HH-3")]);
    answers.set("causes|HH-3", [cand("I3", "HH-3")]);
    answers.set("blocked by|HH-3", [cand("I3", "HH-3", "すでに blocked by の関連があります")]);
    await relations.load("S1", "I1");
    await relations.setQuery("hh");
    relations.choose(first(get(relations.state).candidates));
    // まだ選べるなら、選んだまま。
    await relations.setLabel("causes");
    expect(calls).toContain("candidates causes HH-3");
    expect(get(relations.state)).toMatchObject({
      label: "causes",
      canAdd: true,
      selected: { id: "I3" },
    });
    // 選べなくなったら、選択を外して、理由つきの候補を見せる。
    await relations.setLabel("blocked by");
    const s = get(relations.state);
    expect(s).toMatchObject({ label: "blocked by", selected: null, canAdd: false, query: "HH-3" });
    expect(s.candidates.map((c) => c.disabledReason)).toEqual([
      "すでに blocked by の関連があります",
    ]);
  });

  it("古い検索の答えは捨てる", async () => {
    const { relations, hold, waiting } = setup();
    hold.add("h");
    hold.add("hh");
    await relations.load("S1", "I1");
    const slow = relations.setQuery("h");
    const fast = relations.setQuery("hh");
    await flush();
    waiting.get("hh")?.([cand("I3", "HH-3")]);
    await fast;
    waiting.get("h")?.([cand("I9", "HH-9")]);
    await slow;
    expect(get(relations.state).candidates.map((c) => c.id)).toEqual(["I3"]);
  });

  it("解除すると、一覧を読み直して画面に知らせる", async () => {
    const { relations, calls, changed } = setup([
      row("blocks", "I2", "HH-2"),
      row("clones", "I3", "HH-3"),
    ]);
    await relations.load("S1", "I1");
    await relations.remove(first(get(relations.state).rows));
    expect(calls).toContain("remove blocks I2");
    expect(get(relations.state).rows.map((r) => r.id)).toEqual(["I3"]);
    expect(changed()).toBe(1);
  });

  it("同じアイテムの読み直しでは入力を残し、別のアイテムでは追加の行を戻す", async () => {
    const { relations, answers } = setup();
    answers.set("blocks|hh", [cand("I3", "HH-3")]);
    await relations.load("S1", "I1");
    await relations.setQuery("hh");
    await relations.load("S1", "I1");
    expect(get(relations.state)).toMatchObject({ query: "hh" });
    expect(get(relations.state).candidates).toHaveLength(1);
    await relations.setLabel("causes");
    await relations.load("S1", "I9");
    expect(get(relations.state)).toMatchObject({
      label: "blocks",
      query: "",
      candidates: [],
      selected: null,
      loaded: true,
    });
  });

  it("追加が通信の失敗なら、失敗のトーストを出し、選んだ相手は残す", async () => {
    const { relations, answers, toasts, failWith, changed } = setup();
    answers.set("blocks|hh", [cand("I3", "HH-3")]);
    await relations.load("S1", "I1");
    await relations.setQuery("hh");
    relations.choose(first(get(relations.state).candidates));
    failWith(new Error("network"));
    expect(await relations.add()).toBe(false);
    expect(get(toasts.list)[0]).toMatchObject({ text: relationFailedMessage, error: true });
    expect(get(relations.state)).toMatchObject({ selected: { id: "I3" }, busy: false });
    expect(changed()).toBe(0);
  });

  it("相手がほかで消えていたら、トーストを出さず、一覧を読み直す", async () => {
    const { relations, answers, toasts, failWith, calls } = setup();
    answers.set("blocks|hh", [cand("I3", "HH-3")]);
    await relations.load("S1", "I1");
    await relations.setQuery("hh");
    relations.choose(first(get(relations.state).candidates));
    failWith(new NotFound("gone"));
    expect(await relations.add()).toBe(false);
    expect(get(toasts.list)).toEqual([]);
    expect(calls.filter((c) => c.startsWith("list")).length).toBe(2);
  });

  it("解除が通信の失敗でも、失敗のトーストを出して読み直す", async () => {
    const { relations, toasts, failWith, calls, changed } = setup();
    await relations.load("S1", "I1");
    failWith(new Error("network"));
    await relations.remove(first(get(relations.state).rows));
    expect(get(toasts.list)[0]).toMatchObject({ text: relationFailedMessage, error: true });
    expect(calls.filter((c) => c.startsWith("list")).length).toBe(2);
    expect(changed()).toBe(0);
  });

  it("追加している間は、もう一度追加しない", async () => {
    const { relations, answers, calls, holdAdds, finishAdd } = setup();
    answers.set("blocks|hh", [cand("I3", "HH-3")]);
    await relations.load("S1", "I1");
    await relations.setQuery("hh");
    relations.choose(first(get(relations.state).candidates));
    holdAdds();
    const first1 = relations.add();
    expect(get(relations.state).busy).toBe(true);
    expect(await relations.add()).toBe(false);
    finishAdd();
    expect(await first1).toBe(true);
    expect(calls.filter((c) => c.startsWith("add")).length).toBe(1);
    expect(get(relations.state).busy).toBe(false);
  });

  it("追加している間に別のアイテムへ移ったら、新しいアイテムの入力を触らない", async () => {
    const { relations, answers, holdAdds, finishAdd } = setup();
    answers.set("blocks|hh", [cand("I3", "HH-3")]);
    await relations.load("S1", "I1");
    await relations.setQuery("hh");
    relations.choose(first(get(relations.state).candidates));
    holdAdds();
    const adding = relations.add();
    await relations.load("S1", "I9");
    await relations.setQuery("zz");
    finishAdd();
    await adding;
    expect(get(relations.state)).toMatchObject({ query: "zz", busy: false });
  });

  it("追加している間に同じアイテムを読み直しても、追加できたら入力を空にする", async () => {
    const { relations, answers, holdAdds, finishAdd } = setup();
    answers.set("blocks|hh", [cand("I3", "HH-3")]);
    await relations.load("S1", "I1");
    await relations.setQuery("hh");
    relations.choose(first(get(relations.state).candidates));
    holdAdds();
    const adding = relations.add();
    await relations.load("S1", "I1");
    finishAdd();
    await adding;
    expect(get(relations.state)).toMatchObject({ query: "", selected: null, busy: false });
  });

  it("reset は、状態を最初に戻す", async () => {
    const { relations } = setup();
    await relations.load("S1", "I1");
    relations.reset();
    expect(get(relations.state)).toMatchObject({ loaded: false, rows: [], query: "" });
  });
});
