import { describe, expect, test } from "vitest";
import { buildRows, type Candidate, type GroupId, more, noMatch, type Row } from "./candidates";

function cands(group: GroupId, n: number): Candidate[] {
  return Array.from({ length: n }, (_, i) => ({
    key: `${group}-${i}`,
    group,
    label: `${group} ${i}`,
    run: () => {},
  }));
}

// 行を読みやすい文字列にする: 見出しは「# 見出し」、押せない行は「- 文」、候補は key。
const show = (rows: Row[]) =>
  rows.map((r) =>
    r.kind === "head" ? `# ${r.text}` : r.kind === "more" ? `- ${r.text}` : r.c.key,
  );
const count = (rows: Row[], group: GroupId) =>
  rows.filter((r) => r.kind === "candidate" && r.c.group === group).length;

describe("buildRows", () => {
  test("群の順はコマンド・このクエリで・選んでいるアイテム・作成・設定・アイテム・プロジェクト・ワークスペース・ビュー。見出しを付ける", () => {
    const rows = buildRows(
      {
        view: cands("view", 1),
        workspace: cands("workspace", 1),
        project: cands("project", 1),
        item: cands("item", 1),
        settings: cands("settings", 1),
        create: cands("create", 1),
        selected: cands("selected", 1),
        query: cands("query", 1),
        command: cands("command", 1),
      },
      "a",
      { selected: "HH-1 に対して" },
    );
    expect(show(rows)).toEqual([
      "# コマンド",
      "command-0",
      "# このクエリで",
      "query-0",
      "# HH-1 に対して",
      "selected-0",
      "# 作成",
      "create-0",
      "# 設定",
      "settings-0",
      "# アイテム",
      "item-0",
      "# プロジェクト",
      "project-0",
      "# ワークスペース",
      "workspace-0",
      "# ビュー",
      "view-0",
    ]);
  });

  test("入力が空のときは、コマンド・選んでいるアイテム・最近開いたものだけを出す(アイテム・プロジェクト・ワークスペース・作成・設定を出さない)", () => {
    const groups = {
      command: cands("command", 1),
      selected: cands("selected", 1),
      create: cands("create", 1),
      settings: cands("settings", 1),
      recent: cands("recent", 1),
      item: cands("item", 1),
      project: cands("project", 1),
      workspace: cands("workspace", 1),
    };
    expect(show(buildRows(groups, "  ", { selected: "HH-1 に対して" }))).toEqual([
      "# コマンド",
      "command-0",
      "# HH-1 に対して",
      "selected-0",
      "# 最近開いたもの",
      "recent-0",
    ]);
    // 入力があるときは最近開いたものを出さない。
    expect(count(buildRows(groups, "x"), "recent")).toBe(0);
  });

  test("プロジェクト・ワークスペース・ビューは 5 件まで、コマンドは 8 件まで。切ったら末尾に「ほか N 件あります…」", () => {
    const rows = buildRows(
      {
        command: cands("command", 10),
        project: cands("project", 7),
        workspace: cands("workspace", 5),
        view: cands("view", 6),
      },
      "a",
    );
    expect(count(rows, "command")).toBe(8);
    expect(count(rows, "project")).toBe(5);
    expect(count(rows, "workspace")).toBe(5);
    expect(count(rows, "view")).toBe(5);
    const s = show(rows);
    expect(s[s.indexOf("command-7") + 1]).toBe(`- ${more(2)}`);
    expect(s[s.indexOf("project-4") + 1]).toBe(`- ${more(2)}`);
    expect(s[s.indexOf("workspace-4") + 1]).toBe("# ビュー");
    expect(s.at(-1)).toBe(`- ${more(1)}`);
    expect(more(2)).toBe("ほか 2 件あります。入力を続けて絞り込んでください");
  });

  test("選んでいるアイテム 4・作成 3・設定 6・最近開いたもの 8・このクエリで 3 件まで", () => {
    const typed = buildRows(
      {
        query: cands("query", 5),
        selected: cands("selected", 5),
        create: cands("create", 5),
        settings: cands("settings", 9),
      },
      "a",
    );
    expect(count(typed, "query")).toBe(3);
    expect(count(typed, "selected")).toBe(4);
    expect(count(typed, "create")).toBe(3);
    expect(count(typed, "settings")).toBe(6);
    expect(count(buildRows({ recent: cands("recent", 10) }, ""), "recent")).toBe(8);
  });

  test("アイテムの上限は 40 −(コマンド+プロジェクト+ワークスペース+ビューの出した件数)、最低 17。小さく固定の群は数えない", () => {
    const alone = buildRows({ item: cands("item", 50) }, "a");
    expect(count(alone, "item")).toBe(40);
    expect(show(alone).at(-1)).toBe(`- ${more(10)}`);

    const others = buildRows(
      {
        command: cands("command", 3),
        selected: cands("selected", 4),
        create: cands("create", 3),
        settings: cands("settings", 6),
        query: cands("query", 3),
        item: cands("item", 50),
        project: cands("project", 9),
        workspace: cands("workspace", 2),
      },
      "a",
    );
    // 40 −(3 + 5 + 2)= 30。プロジェクトは 9 件あっても出したのは 5 件。
    expect(count(others, "item")).toBe(30);

    const full = buildRows(
      {
        command: cands("command", 8),
        item: cands("item", 50),
        project: cands("project", 5),
        workspace: cands("workspace", 5),
        view: cands("view", 5),
      },
      "a",
    );
    expect(count(full, "item")).toBe(17);
    // 上限ちょうどなら「ほか N 件」を出さない。
    expect(show(buildRows({ item: cands("item", 40) }, "a")).some((t) => t.startsWith("- "))).toBe(
      false,
    );
  });

  test("何も出す候補がないとき「一致するものはありません」の 1 行", () => {
    expect(show(buildRows({ item: [], recent: cands("recent", 2) }, "zz"))).toEqual([
      `- ${noMatch}`,
    ]);
    expect(show(buildRows({}, ""))).toEqual([`- ${noMatch}`]);
    expect(noMatch).toBe("一致するものはありません");
  });
});
