import { describe, expect, it } from "vitest";
import type { RelationCandidate } from "../../model/relation";
import { get } from "../../reactive/store";
import { flush } from "../items/fixtures.test-util";
import { groupRelations } from "./relations";
import { cand, first, row, setup } from "./relationsFixtures.test-util";

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

  it("reload は、開いていれば一覧だけ読み直し、開いていなければ何もしない", async () => {
    const { relations, calls, answers } = setup();
    await relations.reload();
    expect(calls).toEqual([]);
    answers.set("blocks|hh", [cand("I3", "HH-3")]);
    await relations.load("S1", "I1");
    await relations.setQuery("hh");
    await relations.reload();
    expect(calls.filter((c) => c.startsWith("list")).length).toBe(2);
    expect(get(relations.state)).toMatchObject({ query: "hh" });
    relations.reset();
    await relations.reload();
    expect(calls.filter((c) => c.startsWith("list")).length).toBe(2);
  });

  it("reset は、状態を最初に戻す", async () => {
    const { relations } = setup();
    await relations.load("S1", "I1");
    relations.reset();
    expect(get(relations.state)).toMatchObject({ loaded: false, rows: [], query: "" });
  });
});
