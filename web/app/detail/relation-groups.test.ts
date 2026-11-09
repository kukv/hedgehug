import { describe, expect, it } from "vitest";
import type { Crumb, Lookup } from "./lookup";
import { addRefBody, buildGroups, relationKinds, removeRefBody } from "./relation-groups";
import { showView } from "./testkit";

const crumbs: Record<string, Crumb> = {
  UB: {
    ulid: "UB",
    id: "REL-2",
    title: "破壊的変更",
    kind: "task",
    status: "waiting",
    parent: null,
    store: "shared",
  },
  UR: {
    ulid: "UR",
    id: "REL-7",
    title: "スキーマ再生成",
    kind: "task",
    status: "in_progress",
    parent: null,
    store: "shared",
  },
  UD: {
    ulid: "UD",
    id: "SH-26",
    title: "対応",
    kind: "task",
    status: "canceled",
    parent: null,
    store: "shared",
  },
  UK: {
    ulid: "UK",
    id: "REL-4",
    title: "移行ガイド",
    kind: "task",
    status: "todo",
    parent: null,
    store: "shared",
  },
};
const lookup: Lookup = async (k) => crumbs[k];

const ref = (ulid: string, id = ulid) => ({ ulid, id, title: ulid });

const show = showView({
  ulid: "U6",
  id: "REL-6",
  blocked_by: [{ ulid: "UB", id: "REL-2", title: "破壊的変更" }],
  relates: [ref("UR", "REL-7")],
  duplicates: [ref("UD", "SH-26")],
  reverse: { duplicated_by: [ref("UD", "SH-26")], caused_by: [ref("UK", "REL-4")] },
  referenced_by: [{ ulid: "UK", id: "REL-4", title: "移行ガイド" }],
});

describe("buildGroups", () => {
  it("5 つの関連、その逆方向、バックリンクの順に、固定の表示名で並べる", async () => {
    const groups = await buildGroups(show, lookup);
    expect(groups.map((g) => g.key)).toEqual([
      "blocked_by",
      "relates",
      "duplicates",
      "reverse:caused_by",
      "reverse:duplicated_by",
      "referenced_by",
    ]);
    expect(groups.map((g) => g.label)).toEqual([
      "Blocked by",
      "Relates to",
      "Duplicates",
      "Caused by",
      "Duplicated by",
      "バックリンク",
    ]);
    expect(groups[0]).toMatchObject({ note: "blocked_by", removable: true });
    expect(groups[0]?.items[0]).toMatchObject({ raw: "UB", id: "REL-2", status: "waiting" });
    expect(groups.find((g) => g.key === "reverse:caused_by")).toMatchObject({
      note: "caused_by",
      removable: false,
    });
    expect(groups.at(-1)).toMatchObject({
      note: "本文・コメントの #REL-6",
      removable: false,
    });
  });

  it("同じ相手を複数の関連に置ける", async () => {
    const both = showView({ causes: [ref("UR")], clones: [ref("UR")] });
    const groups = await buildGroups(both, lookup);
    expect(groups.map((g) => g.key)).toEqual(["causes", "clones"]);
  });

  it("読めないタスクは持っている参照のまま出す", async () => {
    const groups = await buildGroups(show, async () => undefined);
    expect(groups[0]?.items[0]).toMatchObject({ id: "REL-2", title: "破壊的変更", status: null });
    expect(groups.find((g) => g.key === "relates")?.items[0]).toMatchObject({
      raw: "UR",
      id: "REL-7",
    });
  });

  it("空のグループは出さない", async () => {
    expect(await buildGroups(showView(), lookup)).toEqual([]);
  });
});

describe("relationKinds", () => {
  it("順方向の 5 つだけを選べる", () => {
    expect(relationKinds()).toEqual([
      { key: "blocked_by", label: "Blocked by" },
      { key: "causes", label: "Causes" },
      { key: "relates", label: "Relates to" },
      { key: "duplicates", label: "Duplicates" },
      { key: "clones", label: "Clones" },
    ]);
  });
});

describe("addRefBody / removeRefBody", () => {
  it("トップレベルの ULID の配列で足す。すでにあれば足さない", () => {
    expect(addRefBody(show, "blocked_by", "UX")).toEqual({ blocked_by: ["UB", "UX"] });
    expect(addRefBody(show, "relates", "UR")).toEqual({ relates: ["UR"] });
    expect(addRefBody(show, "clones", "UX")).toEqual({ clones: ["UX"] });
  });
  it("外すと残りの配列を返す。最後の 1 つなら空の配列", () => {
    expect(removeRefBody(show, "blocked_by", "UB")).toEqual({ blocked_by: [] });
    expect(removeRefBody(show, "duplicates", "UD")).toEqual({ duplicates: [] });
  });
});
