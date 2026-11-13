import { describe, expect, it } from "vitest";
import type { Member } from "../../model/member";
import type { SprintCard } from "../../model/planning";
import type { WorkspaceDetail } from "../../model/workspace";
import { get } from "../../reactive/store";
import { createSprints, type SprintsDeps } from "./sprints";

// スプリントの画面のカードの読み込み(createSprints)。sprints.test.ts が 300 行を超えるので分けた。

const card = (): SprintCard => ({
  sprint: {
    id: "SP4",
    name: "Sprint 4",
    goal: "",
    start: "2026-11-02",
    end: "2026-11-15",
    state: "active",
    confirmed: null,
  },
  total: { count: 0, points: 0 },
  done: { count: 0, points: 0 },
  undone: 0,
  canceled: 0,
  items: [],
});

const members: Member[] = [{ login: "kukv", name: "Koki" }];
const detail = { settings: { statuses: [] } } as unknown as WorkspaceDetail;

// later は、あとで決める答え。
function later<T>() {
  let resolve: (v: T) => void = () => {};
  let reject: (e: Error) => void = () => {};
  const promise = new Promise<T>((ok, ng) => {
    resolve = ok;
    reject = ng;
  });
  return { promise, resolve, reject };
}

// depsOf は、カードを answers の順に返す口。設定とメンバーはいつも同じ値を返す。
function depsOf(answers: Promise<SprintCard[]>[]) {
  const calls: string[] = [];
  const deps: SprintsDeps = {
    port: {
      sprintCards: (s, w) => {
        calls.push(`${s}/${w}`);
        const next = answers.shift();
        if (!next) throw new Error("no answer");
        return next;
      },
    },
    workspaces: { get: () => Promise.resolve(detail) },
    listMembers: () => Promise.resolve(members),
  };
  return { deps, calls };
}

describe("createSprints", () => {
  it("reads the cards, the settings and the members of the workspace it opens", async () => {
    const { deps, calls } = depsOf([Promise.resolve([card()])]);
    const s = createSprints(deps);
    await s.open("S1", "W1");
    expect(calls).toEqual(["S1/W1"]);
    expect(get(s.state)).toEqual({
      storeId: "S1",
      workspaceId: "W1",
      cards: [card()],
      settings: detail.settings,
      members,
    });
  });

  it("does nothing on reload before opening", async () => {
    const { deps, calls } = depsOf([]);
    await createSprints(deps).reload();
    expect(calls).toEqual([]);
  });

  it("clears the old workspace while reading another", async () => {
    const second = later<SprintCard[]>();
    const { deps } = depsOf([Promise.resolve([card()]), second.promise]);
    const s = createSprints(deps);
    await s.open("S1", "W1");
    const opening = s.open("S1", "W2");
    expect(get(s.state)).toBeNull();
    second.resolve([]);
    await opening;
    expect(get(s.state)?.workspaceId).toBe("W2");
  });

  it("uses only the last of overlapping reads", async () => {
    const first = later<SprintCard[]>();
    const { deps } = depsOf([first.promise, Promise.resolve([])]);
    const s = createSprints(deps);
    const opening = s.open("S1", "W1");
    await s.reload();
    first.resolve([card()]);
    await opening;
    expect(get(s.state)?.cards).toEqual([]);
  });

  it("keeps the last cards when a reload fails, and does not throw", async () => {
    const failing = later<SprintCard[]>();
    const { deps } = depsOf([Promise.resolve([card()]), failing.promise]);
    const s = createSprints(deps);
    await s.open("S1", "W1");
    const reloading = s.reload();
    failing.reject(new Error("offline"));
    await expect(reloading).resolves.toBeUndefined();
    expect(get(s.state)?.cards).toEqual([card()]);
  });

  it("stops reading after close", async () => {
    const first = later<SprintCard[]>();
    const { deps, calls } = depsOf([first.promise]);
    const s = createSprints(deps);
    const opening = s.open("S1", "W1");
    s.close();
    first.resolve([card()]);
    await opening;
    await s.reload();
    expect(calls).toEqual(["S1/W1"]);
    expect(get(s.state)).toBeNull();
  });
});
