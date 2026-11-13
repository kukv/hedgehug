import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { fakeItemActions } from "./actions.test-util";
import { fakeItemsPort, flush, gate, item } from "./fixtures.test-util";
import { createItemKind, kindFailedMessage, kindOptions } from "./kind";

// HH-1(Epic)の下に HH-2(Task)、その下に HH-3(Step)。閲覧のみの Store の Task は S2 に置く。
const epic = item("S1", "S1-P1", 1, "Epic", null, "認証");
const task = item("S1", "S1-P1", 2, "Task", epic.id, "画面");
const step = item("S1", "S1-P1", 3, "Step", task.id, "設計");
const ro = item("S2", "S2-P1", 1, "Task", null, "閲覧のみ");

async function setup() {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const actions = fakeItemActions();
  const items = fakeItemsPort([epic, task, step]);
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const refreshed: string[] = [];
  const kind = createItemKind({
    actions,
    items,
    catalog,
    toasts,
    refresh: {
      afterChange: async () => {
        refreshed.push("change");
      },
    },
  });
  const messages = () => get(toasts.list).map((t) => `${t.error ? "!" : ""}${t.text}`);
  return { kind, actions, items, refreshed, messages };
}

describe("kindOptions", () => {
  it("offers the other two of Story, Task and Bug, all three for a Step, and nothing for an Epic", () => {
    expect(kindOptions("Task")).toEqual(["Story", "Bug"]);
    expect(kindOptions("Story")).toEqual(["Task", "Bug"]);
    expect(kindOptions("Bug")).toEqual(["Story", "Task"]);
    expect(kindOptions("Step")).toEqual(["Story", "Task", "Bug"]);
    expect(kindOptions("Epic")).toEqual([]);
  });
});

describe("createItemKind", () => {
  it("opens the kind menu at the given place", async () => {
    const { kind } = await setup();
    kind.open(task, 10, 20);
    expect(get(kind.state)).toEqual({
      step: "menu",
      item: task,
      x: 10,
      y: 20,
      options: ["Story", "Bug"],
    });
  });

  it("does nothing for an Epic or in a read-only Store", async () => {
    const { kind } = await setup();
    kind.open(epic, 0, 0);
    expect(get(kind.state)).toBeNull();
    kind.open(ro, 0, 0);
    expect(get(kind.state)).toBeNull();
  });

  it("asks for confirmation after a kind is chosen", async () => {
    const { kind, actions } = await setup();
    kind.open(task, 0, 0);
    await kind.choose("Bug");
    expect(get(kind.state)).toEqual({
      step: "confirm",
      item: task,
      to: "Bug",
      title: "種類を変更",
      lines: ["HH-2 の種類を Task から Bug に変更します。"],
      ok: "変更",
    });
    expect(actions.calls).toEqual([]);
  });

  it("says where a Step goes: under the parent of its parent", async () => {
    const { kind } = await setup();
    kind.open(step, 0, 0);
    await kind.choose("Story");
    const s = get(kind.state);
    expect(s?.step === "confirm" && s.lines).toEqual([
      "HH-3 の種類を Step から Story に変更します。HH-1 の下に移ります。",
    ]);
  });

  it("ignores a choice that is not in the menu (a Task cannot become a Step)", async () => {
    const { kind } = await setup();
    await kind.choose("Bug");
    expect(get(kind.state)).toBeNull();
    kind.open(task, 0, 0);
    await kind.choose("Step");
    await kind.choose("Task");
    expect(get(kind.state)).toMatchObject({ step: "menu" });
  });

  it("changes the kind on confirm, with no toast, and reloads", async () => {
    const { kind, actions, refreshed, messages } = await setup();
    kind.open(task, 0, 0);
    await kind.choose("Bug");
    await kind.confirm();
    expect(actions.calls).toEqual([`changeKind ${task.id} Bug`]);
    expect(get(kind.state)).toBeNull();
    expect(messages()).toEqual([]);
    expect(refreshed).toEqual(["change"]);
  });

  it("changes nothing on cancel, from the menu or from the dialog", async () => {
    const { kind, actions, refreshed } = await setup();
    kind.open(task, 0, 0);
    kind.cancel();
    expect(get(kind.state)).toBeNull();
    kind.open(task, 0, 0);
    await kind.choose("Bug");
    kind.cancel();
    await kind.confirm();
    expect(get(kind.state)).toBeNull();
    expect(actions.calls).toEqual([]);
    expect(refreshed).toEqual([]);
  });

  it("sends one change for a double confirm", async () => {
    const { kind, actions } = await setup();
    kind.open(task, 0, 0);
    await kind.choose("Bug");
    const hold = actions.holdNext();
    const first = kind.confirm();
    await flush();
    await kind.confirm();
    hold.release();
    await first;
    expect(actions.calls).toEqual([`changeKind ${task.id} Bug`]);
  });

  it("shows a refusal as an error toast and does not reload", async () => {
    const { kind, actions, refreshed, messages } = await setup();
    kind.open(task, 0, 0);
    await kind.choose("Bug");
    actions.refuseNext("Bug は変更できません");
    await kind.confirm();
    expect(messages()).toEqual(["!Bug は変更できません"]);
    expect(get(kind.state)).toBeNull();
    expect(refreshed).toEqual([]);
  });

  it("shows another failure with the failure message", async () => {
    const { kind, actions, messages } = await setup();
    kind.open(task, 0, 0);
    await kind.choose("Bug");
    actions.failNext();
    await kind.confirm();
    expect(messages()).toEqual([`!${kindFailedMessage}`]);
  });

  it("reloads without a toast when the item is already gone", async () => {
    const { kind, actions, refreshed, messages } = await setup();
    kind.open(task, 0, 0);
    await kind.choose("Bug");
    actions.missing.add(task.id);
    await kind.confirm();
    expect(messages()).toEqual([]);
    expect(refreshed).toEqual(["change"]);
  });

  it("drops the parent lookup of a Step when the menu was cancelled meanwhile", async () => {
    const { kind, items } = await setup();
    const g = gate();
    const read = items.list.bind(items);
    items.list = async (s, p) => {
      await g.wait;
      return read(s, p);
    };
    kind.open(step, 0, 0);
    const choosing = kind.choose("Story");
    await flush();
    kind.cancel();
    g.release();
    await choosing;
    expect(get(kind.state)).toBeNull();
  });

  it("reloads instead of asking when the parent of a Step's parent cannot be found", async () => {
    const { kind, items, refreshed } = await setup();
    items.all.splice(0, 1);
    kind.open(step, 0, 0);
    await kind.choose("Story");
    expect(get(kind.state)).toBeNull();
    expect(refreshed).toEqual(["change"]);
  });
});
