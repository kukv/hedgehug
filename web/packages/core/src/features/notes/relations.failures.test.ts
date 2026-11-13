import { describe, expect, it } from "vitest";
import { NotFound } from "../../model/errors";
import { get } from "../../reactive/store";
import { relationFailedMessage } from "./relations";
import { cand, first, setup } from "./relationsFixtures.test-util";

describe("createRelations: 書き込みの失敗と途中の切り替え", () => {
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
});
