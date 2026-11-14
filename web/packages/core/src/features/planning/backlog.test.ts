import { describe, expect, it } from "vitest";
import type { Member } from "../../model/member";
import type { BacklogFrame } from "../../model/planning";
import { get } from "../../reactive/store";
import { fakeItemsPort, gate, item } from "../items/fixtures.test-util";
import { fakeWorkspaces, settings } from "../items/settings.test-util";
import { createToasts } from "../toasts/toasts";
import { createBacklog } from "./backlog";
import { fakePlanningPort, frameOf, rowOf, sprintOf } from "./fixtures.test-util";

// バックログの画面の状態と保存(05「バックログ」、F2a-8・F2a-9・F2a-10)。
// 行の ID は、偽物のアイテムの口の ID(S1-I{番号})。表示の ID は HH-{番号}。

const members: Member[] = [{ login: "kukv", name: "Koki" }];
const sp1 = sprintOf({ id: "SP1", state: "active" });
const row = (n: number, status: string, points: number | null = null) =>
  rowOf({ id: `S1-I${n}`, display: `HH-${n}`, status, points });

// framesOf は、スプリント SP1 の枠(HH-5 中止・HH-6 完了)とバックログの枠(HH-3 未着手・HH-4 進行中)。
const framesOf = (): BacklogFrame[] => [
  frameOf(sp1, [row(5, "st4"), row(6, "st3")]),
  frameOf(null, [row(3, "st1", 2), row(4, "st2")]),
];

async function setup() {
  const planning = fakePlanningPort();
  planning.backlogs["S1-W1"] = framesOf();
  planning.backlogs["S1-W2"] = [];
  const items = fakeItemsPort([3, 4, 5, 6].map((n) => item("S1", "S1-P1", n, "Task", null)));
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const opened: string[] = [];
  const backlog = createBacklog({
    port: planning,
    items,
    workspaces: fakeWorkspaces(),
    listMembers: async () => members,
    toasts,
    openDetail: (s, id) => void opened.push(`${s}/${id}`),
  });
  await backlog.open("S1", "S1-W1");
  planning.calls.length = 0;
  const shown = () => get(toasts.list).map((t) => ({ text: t.text, error: t.error }));
  return { backlog, planning, items, shown, opened };
}

const frames = (b: Awaited<ReturnType<typeof setup>>["backlog"]) => get(b.state)?.frames ?? [];
const rowIn = (b: Awaited<ReturnType<typeof setup>>["backlog"], n: number) => {
  const r = frames(b)
    .flatMap((f) => f.rows)
    .find((x) => x.id === `S1-I${n}`);
  if (!r) throw new Error(`no row ${n}`);
  return r;
};

describe("createBacklog reading", () => {
  it("reads the frames, the settings and the members of the workspace it opens", async () => {
    const { backlog } = await setup();
    expect(get(backlog.state)).toEqual({
      storeId: "S1",
      workspaceId: "S1-W1",
      frames: framesOf(),
      settings,
      members,
    });
  });

  it("clears the old workspace while reading another", async () => {
    const { backlog } = await setup();
    const opening = backlog.open("S1", "S1-W2");
    expect(get(backlog.state)).toBeNull();
    await opening;
    expect(get(backlog.state)?.workspaceId).toBe("S1-W2");
  });

  it("uses only the last of overlapping reads, and keeps the frames when a read fails", async () => {
    const { backlog, planning } = await setup();
    const g = gate();
    const old = planning.backlog;
    planning.backlog = async (s, w) => {
      await g.wait;
      return old(s, w);
    };
    const first = backlog.reload();
    planning.backlog = async () => [];
    await backlog.reload();
    g.release();
    await first;
    expect(frames(backlog)).toEqual([]);
    planning.backlog = () => Promise.reject(new Error("offline"));
    await expect(backlog.reload()).resolves.toBeUndefined();
    expect(frames(backlog)).toEqual([]);
  });
});

describe("createBacklog status", () => {
  it("saves the status and tells that a done backlog item disappears", async () => {
    const { backlog, items, planning, shown } = await setup();
    await backlog.setStatus(rowIn(backlog, 3), "st3");
    expect(items.calls).toEqual(['setProperty S1-I3 status "st3"']);
    expect(shown()).toEqual([
      { text: "HH-3 は完了のため、バックログには表示されません", error: false },
    ]);
    expect(planning.calls).toEqual(["backlog S1-W1"]);
  });

  it("does not tell it for a row of a sprint frame or a status that stays in the backlog", async () => {
    const { backlog, shown } = await setup();
    await backlog.setStatus(rowIn(backlog, 5), "st3");
    await backlog.setStatus(rowIn(backlog, 3), "st2");
    expect(shown()).toEqual([]);
  });

  it("shows a refusal or 保存できませんでした as an error toast and reloads", async () => {
    const { backlog, items, planning, shown } = await setup();
    items.refuseNext("このステータスには変更できません");
    await backlog.setStatus(rowIn(backlog, 3), "st3");
    await backlog.setStatus(rowOf({ id: "S1-I9", display: "HH-9" }), "st3");
    expect(shown()).toEqual([
      { text: "このステータスには変更できません", error: true },
      { text: "保存できませんでした", error: true },
    ]);
    expect(planning.calls).toEqual(["backlog S1-W1", "backlog S1-W1"]);
  });
});

describe("createBacklog points", () => {
  it("sends the points as text and does not send the same value", async () => {
    const { backlog, items, planning } = await setup();
    expect(await backlog.setPoints(rowIn(backlog, 3), "2")).toBe(true);
    expect(await backlog.setPoints(rowIn(backlog, 4), "")).toBe(true);
    expect(items.calls).toEqual([]);
    expect(await backlog.setPoints(rowIn(backlog, 4), "3")).toBe(true);
    expect(items.calls).toEqual(['setProperty S1-I4 points "3"']);
    expect(planning.calls).toEqual(["backlog S1-W1"]);
  });

  it("shows a refusal as an error toast and tells the input to revert", async () => {
    const { backlog, items, planning, shown } = await setup();
    items.refuseNext("ポイントは 0 以上の数で入力してください");
    expect(await backlog.setPoints(rowIn(backlog, 3), "-1")).toBe(false);
    expect(shown()).toEqual([{ text: "ポイントは 0 以上の数で入力してください", error: true }]);
    expect(planning.calls).toEqual(["backlog S1-W1"]);
  });
});

describe("createBacklog sprint", () => {
  it("moves a done row of a sprint frame to the backlog and tells that it disappears", async () => {
    const { backlog, planning, shown } = await setup();
    await backlog.setSprint(rowIn(backlog, 6), "");
    expect(planning.calls).toEqual(["sprint S1-I6 null", "backlog S1-W1"]);
    expect(shown()).toEqual([
      { text: "HH-6 は完了のため、バックログには表示されません", error: false },
    ]);
  });

  it("does not send the same value, and does not tell it for an open item", async () => {
    const { backlog, planning, shown } = await setup();
    await backlog.setSprint(rowIn(backlog, 6), "SP1");
    await backlog.setSprint(rowIn(backlog, 3), "");
    expect(planning.calls).toEqual([]);
    await backlog.setSprint(rowIn(backlog, 3), "SP1");
    expect(planning.calls).toEqual(["sprint S1-I3 SP1", "backlog S1-W1"]);
    expect(shown()).toEqual([]);
  });

  it("shows a refusal as an error toast and reloads", async () => {
    const { backlog, planning, shown } = await setup();
    planning.refuseNext("完了したスプリントには入れられません");
    await backlog.setSprint(rowIn(backlog, 3), "SP0");
    expect(shown()).toEqual([{ text: "完了したスプリントには入れられません", error: true }]);
    expect(planning.calls).toEqual(["backlog S1-W1"]);
  });
});

describe("createBacklog place", () => {
  it("drops a canceled row of a sprint frame into the backlog and tells that it disappears", async () => {
    const { backlog, planning, shown } = await setup();
    await backlog.place(rowIn(backlog, 5), { sprint: null, anchor: "S1-I4", after: false });
    expect(planning.calls).toEqual(["place S1-I5 null S1-I4 false", "backlog S1-W1"]);
    expect(shown()).toEqual([
      { text: "HH-5 は中止のため、バックログには表示されません", error: false },
    ]);
  });

  it("does not tell it inside the sprint frames or for an open item", async () => {
    const { backlog, planning, shown } = await setup();
    await backlog.place(rowIn(backlog, 6), { sprint: "SP1", anchor: "S1-I5", after: false });
    await backlog.place(rowIn(backlog, 4), { sprint: null, anchor: "S1-I3", after: false });
    expect(planning.calls).toEqual([
      "place S1-I6 SP1 S1-I5 false",
      "backlog S1-W1",
      "place S1-I4 null S1-I3 false",
      "backlog S1-W1",
    ]);
    expect(shown()).toEqual([]);
  });

  it("shows a refusal or 保存できませんでした as an error toast and reloads", async () => {
    const { backlog, planning, shown } = await setup();
    planning.refuseNext("完了したスプリントには入れられません");
    await backlog.place(rowIn(backlog, 3), { sprint: "SP0", anchor: null, after: false });
    planning.failNext(new Error("offline"));
    await backlog.place(rowIn(backlog, 3), { sprint: "SP1", anchor: null, after: false });
    expect(shown()).toEqual([
      { text: "完了したスプリントには入れられません", error: true },
      { text: "保存できませんでした", error: true },
    ]);
    expect(planning.calls).toEqual(["backlog S1-W1", "backlog S1-W1"]);
  });
});

describe("createBacklog detail", () => {
  const done = (n: number, sprint = "") => ({
    ...item("S1", "S1-P1", n, "Task", null),
    status: "st3",
    sprint,
  });

  it("opens the detail panel of the row", async () => {
    const { backlog, opened } = await setup();
    backlog.openDetail(rowIn(backlog, 3));
    expect(opened).toEqual(["S1/S1-I3"]);
  });

  it("tells that the item opened from the screen disappears when it is done outside the sprints", async () => {
    const { backlog, shown } = await setup();
    backlog.openDetail(rowIn(backlog, 3));
    backlog.noticeDetail("S1", done(3));
    backlog.noticeDetail("S1", done(3));
    expect(shown()).toEqual([
      { text: "HH-3 は完了のため、バックログには表示されません", error: false },
    ]);
  });

  it("does not tell it for another item, an item in a sprint, an open item or after close", async () => {
    const { backlog, shown } = await setup();
    backlog.openDetail(rowIn(backlog, 3));
    backlog.noticeDetail("S1", done(4));
    backlog.noticeDetail("S1", done(3, "SP1"));
    backlog.noticeDetail("S1", { ...done(3), status: "st2" });
    backlog.close();
    await backlog.open("S1", "S1-W1");
    backlog.noticeDetail("S1", done(3));
    expect(shown()).toEqual([]);
  });
});
