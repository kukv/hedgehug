import { describe, expect, it } from "vitest";
import type { Crumb, Lookup } from "./lookup";
import { addRefBody, buildGroups, relationKinds, removeRefBody } from "./relation-groups";
import { schemaView, showView } from "./testkit";

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

const show = showView({
  ulid: "U6",
  id: "REL-6",
  blocked_by: [{ ulid: "UB", id: "REL-2", title: "破壊的変更" }],
  props: { relates: ["UR"], duplicates: "UD" },
  reverse: { duplicates: [{ ulid: "UD", id: "SH-26", title: "対応" }] },
  referenced_by: [{ ulid: "UK", id: "REL-4", title: "移行ガイド" }],
});

describe("buildGroups", () => {
  it("makes a group for blocked_by, each task_ref property, the reverse side and the backlinks", async () => {
    const groups = await buildGroups(show, schemaView, lookup);
    expect(groups.map((g) => g.key)).toEqual([
      "blocked_by",
      "duplicates",
      "relates",
      "reverse:duplicates",
      "referenced_by",
    ]);
    const blocked = groups[0];
    expect(blocked).toMatchObject({
      label: "ブロックされている",
      note: "blocked_by",
      removable: true,
    });
    expect(blocked?.items[0]).toMatchObject({ raw: "UB", id: "REL-2", status: "waiting" });
    expect(groups.find((g) => g.key === "relates")).toMatchObject({
      label: "関連",
      note: "relates（双方向）",
    });
    expect(groups.find((g) => g.key === "reverse:duplicates")).toMatchObject({
      label: "重複されている",
      removable: false,
    });
    expect(groups.at(-1)).toMatchObject({
      label: "バックリンク",
      note: "本文・コメントの #REL-6",
      removable: false,
    });
  });

  it("falls back to the reference itself when the task cannot be read", async () => {
    const groups = await buildGroups(show, schemaView, async () => undefined);
    expect(groups[0]?.items[0]).toMatchObject({ id: "REL-2", title: "破壊的変更", status: null });
    expect(groups.find((g) => g.key === "relates")?.items[0]).toMatchObject({
      raw: "UR",
      id: "UR",
    });
  });

  it("leaves out empty groups", async () => {
    expect(await buildGroups(showView(), schemaView, lookup)).toEqual([]);
  });
});

describe("relationKinds", () => {
  it("offers blocked_by and the task_ref properties", () => {
    expect(relationKinds(schemaView).map((k) => k.key)).toEqual([
      "blocked_by",
      "duplicates",
      "relates",
    ]);
  });
});

describe("addRefBody / removeRefBody", () => {
  it("adds to blocked_by by ULID", () => {
    expect(addRefBody(show, schemaView, "blocked_by", "UX")).toEqual({ blocked_by: ["UB", "UX"] });
    expect(removeRefBody(show, schemaView, "blocked_by", "UB")).toEqual({ blocked_by: [] });
  });
  it("adds to a multiple property as a list and to a single one as a value", () => {
    expect(addRefBody(show, schemaView, "relates", "UX")).toEqual({
      props: { relates: ["UR", "UX"] },
    });
    expect(addRefBody(show, schemaView, "duplicates", "UX")).toEqual({
      props: { duplicates: "UX" },
    });
  });
  it("removes from a list, and clears a single value with null", () => {
    expect(removeRefBody(show, schemaView, "relates", "UR")).toEqual({ props: { relates: [] } });
    expect(removeRefBody(show, schemaView, "duplicates", "UD")).toEqual({
      props: { duplicates: null },
    });
  });
});
