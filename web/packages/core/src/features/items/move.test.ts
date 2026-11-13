import { describe, expect, it } from "vitest";
import type { MoveCandidate } from "../../model/itemAction";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { fakeItemActions } from "./actions.test-util";
import { flush, gate, item } from "./fixtures.test-util";
import { createItemMove, moveFailedMessage } from "./move";

// 「プロジェクト 1」(S1-P1)の Task HH-2 を動かす。候補は、同じプロジェクトの Epic HH-1、別のプロジェクトの Epic HH-9。
const epic: MoveCandidate = {
  kind: "item",
  id: "S1-I1",
  display: "HH-1",
  title: "認証",
  projectId: "S1-P1",
  projectTitle: "プロジェクト 1",
};
const far: MoveCandidate = {
  kind: "item",
  id: "S1-I9",
  display: "HH-9",
  title: "決済",
  projectId: "S1-P2",
  projectTitle: "プロジェクト 2",
};
const project: MoveCandidate = {
  kind: "project",
  id: "S1-P2",
  display: "HH-P2",
  title: "プロジェクト 2",
  projectId: "S1-P2",
  projectTitle: "",
};

async function setup() {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const actions = fakeItemActions();
  actions.candidates = [epic, far];
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const refreshed: string[] = [];
  const move = createItemMove({
    actions,
    catalog,
    toasts,
    refresh: {
      afterChange: async () => {
        refreshed.push("change");
      },
    },
  });
  const task = item("S1", "S1-P1", 2, "Task", "S1-I1", "画面");
  const ro = item("S2", "S2-P1", 1, "Task", null, "閲覧のみ");
  const messages = () => get(toasts.list).map((t) => `${t.error ? "!" : ""}${t.text}`);
  return { move, actions, refreshed, messages, task, ro };
}

describe("createItemMove", () => {
  it("opens at the given place, loads the candidates, and filters them by the query", async () => {
    const { move, actions, task } = await setup();
    const opening = move.open(task, 10, 20);
    expect(get(move.state)).toMatchObject({ x: 10, y: 20, loading: true, shown: [] });
    await opening;
    expect(actions.calls).toEqual([`moveCandidates ${task.id}`]);
    expect(get(move.state)).toMatchObject({ loading: false, query: "", shown: [epic, far] });
    move.setQuery("決済");
    expect(get(move.state)?.shown).toEqual([far]);
    move.setQuery("hh-1");
    expect(get(move.state)?.shown).toEqual([epic]);
    move.setQuery("");
    expect(get(move.state)?.shown).toEqual([epic, far]);
  });

  it("does nothing in a read-only Store", async () => {
    const { move, actions, ro } = await setup();
    await move.open(ro, 0, 0);
    expect(get(move.state)).toBeNull();
    expect(actions.calls).toEqual([]);
  });

  it("moves under an item in the same project and says so by the new parent's ID", async () => {
    const { move, actions, refreshed, messages, task } = await setup();
    await move.open(task, 0, 0);
    await move.choose(epic);
    expect(actions.calls.at(-1)).toBe(`move ${task.id} {"parent":"S1-I1"}`);
    expect(get(move.state)).toBeNull();
    expect(messages()).toEqual(["HH-1 の下に移動しました"]);
    expect(refreshed).toEqual(["change"]);
  });

  it("says the project name when the project changes, comparing project IDs", async () => {
    const { move, actions, messages, task } = await setup();
    await move.moveTo(task, far);
    expect(actions.calls.at(-1)).toBe(`move ${task.id} {"parent":"S1-I9"}`);
    expect(messages()).toEqual(["プロジェクト 2 に移動しました"]);
    const epicItem = item("S1", "S1-P1", 1, "Epic", null, "認証");
    await move.moveTo(epicItem, project);
    expect(actions.calls.at(-1)).toBe(`move ${epicItem.id} {"project":"S1-P2"}`);
    expect(messages().at(-1)).toBe("プロジェクト 2 に移動しました");
    // 名前が同じでも、プロジェクトの内部 ID が違えばプロジェクト名の文。
    await move.moveTo(task, { ...epic, projectId: "S1-P7" });
    expect(messages().at(-1)).toBe("プロジェクト 1 に移動しました");
  });

  it("moveTo does nothing in a read-only Store", async () => {
    const { move, actions, ro, messages } = await setup();
    expect(await move.moveTo(ro, epic)).toBe(false);
    expect(actions.calls).toEqual([]);
    expect(messages()).toEqual([]);
  });

  it("keeps the candidates of the later open when an earlier open answers last", async () => {
    const { move, actions, task } = await setup();
    const other = item("S1", "S1-P1", 3, "Task", "S1-I1", "別");
    const a = gate();
    const b = gate();
    actions.moveCandidates = async (_s, id) => {
      await (id === task.id ? a : b).wait;
      return id === task.id ? [epic] : [far];
    };
    const openA = move.open(task, 0, 0);
    const openB = move.open(other, 0, 0);
    b.release();
    await openB;
    a.release();
    await openA;
    move.setQuery("");
    expect(get(move.state)?.item).toBe(other);
    expect(get(move.state)?.shown).toEqual([far]);
  });

  it("moveTo returns whether the move happened", async () => {
    const { move, actions, task } = await setup();
    expect(await move.moveTo(task, epic)).toBe(true);
    actions.refuseNext("だめ");
    expect(await move.moveTo(task, epic)).toBe(false);
    actions.failNext();
    expect(await move.moveTo(task, epic)).toBe(false);
  });

  it("moveTo refuses a second move while the first is in progress", async () => {
    const { move, actions, task } = await setup();
    const hold = actions.holdNext();
    const first = move.moveTo(task, epic);
    await flush();
    expect(await move.moveTo(task, far)).toBe(false);
    hold.release();
    expect(await first).toBe(true);
    expect(actions.calls.filter((c) => c.startsWith("move "))).toEqual([
      `move ${task.id} {"parent":"S1-I1"}`,
    ]);
  });

  it("shows a refusal as an error toast with its words, and does not reload", async () => {
    const { move, actions, refreshed, messages, task } = await setup();
    await move.open(task, 0, 0);
    actions.refuseNext("Task は Task の下に置けません");
    await move.choose(epic);
    expect(messages()).toEqual(["!Task は Task の下に置けません"]);
    expect(refreshed).toEqual([]);
    expect(get(move.state)).toBeNull();
  });

  it("shows another failure with the failure words", async () => {
    const { move, actions, messages, task } = await setup();
    await move.open(task, 0, 0);
    actions.failNext();
    await move.choose(epic);
    expect(messages()).toEqual([`!${moveFailedMessage}`]);
  });

  it("reloads when the item or the target is already gone", async () => {
    const { move, actions, refreshed, messages, task } = await setup();
    await move.open(task, 0, 0);
    actions.missing.add(task.id);
    await move.choose(epic);
    expect(messages()).toEqual([`!${moveFailedMessage}`]);
    expect(refreshed).toEqual(["change"]);
    // 候補を読むときに消えていたら、メニューを閉じて読み直す(トーストなし)。
    await move.open(task, 0, 0);
    expect(get(move.state)).toBeNull();
    expect(refreshed).toEqual(["change", "change"]);
    expect(messages()).toHaveLength(1);
  });

  it("sends one move for a double choose, and ignores a choose while loading", async () => {
    const { move, actions, task } = await setup();
    const g = gate();
    const read = actions.moveCandidates.bind(actions);
    actions.moveCandidates = async (s, id) => {
      await g.wait;
      return read(s, id);
    };
    const opening = move.open(task, 0, 0);
    await flush();
    await move.choose(epic);
    expect(actions.calls.filter((c) => c.startsWith("move "))).toEqual([]);
    g.release();
    await opening;
    const hold = actions.holdNext();
    const first = move.choose(epic);
    await flush();
    await move.choose(far);
    hold.release();
    await first;
    expect(actions.calls.filter((c) => c.startsWith("move "))).toEqual([
      `move ${task.id} {"parent":"S1-I1"}`,
    ]);
  });

  it("drops candidates that arrive after the menu was closed", async () => {
    const { move, actions, task } = await setup();
    const g = gate();
    const read = actions.moveCandidates.bind(actions);
    actions.moveCandidates = async (s, id) => {
      await g.wait;
      return read(s, id);
    };
    const opening = move.open(task, 0, 0);
    await flush();
    move.close();
    g.release();
    await opening;
    expect(get(move.state)).toBeNull();
  });
});
