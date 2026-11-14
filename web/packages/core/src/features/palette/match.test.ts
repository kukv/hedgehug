import { describe, expect, test } from "vitest";
import type { PaletteAlias, PaletteItem, PaletteProject } from "../../model/palette";
import type { StoreNode, WorkspaceNode } from "../catalog/catalog";
import { byName, matchItems, matchProjects, matchWorkspaces } from "./match";

// 並び: Store「work」(ワークスペース Web(HH)→ Mobile(MOB))→ Store「ro」(ワークスペース Web(HH))。
function ws(storeId: string, id: string, name: string, prefix: string, projects: string[]) {
  return {
    id,
    storeId,
    name,
    prefix,
    oldPrefixes: [],
    projects: projects.map((p, i) => ({
      id: p,
      storeId,
      workspaceId: id,
      number: i + 1,
      rank: `V${i}`,
      title: p,
      owner: "",
      status: "Todo",
      start: "",
      end: "",
      repos: [],
      displayId: `${prefix}-P${i + 1}`,
    })),
  } as WorkspaceNode;
}
const stores: StoreNode[] = [
  {
    id: "S1",
    name: "work",
    color: "teal",
    readOnly: false,
    workspaces: [
      ws("S1", "W1", "Web", "HH", ["P1", "P2"]),
      ws("S1", "W2", "Mobile", "MOB", ["P3", "P5"]),
    ],
  },
  {
    id: "S2",
    name: "ro",
    color: "orange",
    readOnly: true,
    workspaces: [ws("S2", "W3", "Web", "HH", ["P4"])],
  },
];

function item(
  storeId: string,
  workspaceId: string,
  display: string,
  title: string,
  aliases: PaletteAlias[] = [],
): PaletteItem {
  const number = Number(display.split("-")[1]);
  return {
    storeId,
    workspaceId,
    projectId: "P1",
    itemId: `${storeId}-${display}`,
    number,
    display,
    title,
    projectTitle: `プロジェクト ${workspaceId}`,
    aliases,
  };
}

function project(
  storeId: string,
  workspaceId: string,
  projectId: string,
  display: string,
  title: string,
  aliases: PaletteAlias[] = [],
): PaletteProject {
  return { storeId, workspaceId, projectId, display, title, workspaceName: "Web", aliases };
}

const now = new Date(2026, 10, 14);
const ids = (r: { c: { display: string } }[]) => r.map((x) => x.c.display);

describe("matchItems", () => {
  test("ID の一致がタイトルの一致より先で、それぞれ Store・ワークスペースの並び・番号の順", () => {
    const index = {
      items: [
        item("S2", "W3", "HH-1", "ログイン"),
        item("S1", "W2", "MOB-2", "hh の話"),
        item("S1", "W1", "HH-12", "画面"),
        item("S1", "W1", "HH-2", "設計"),
        item("S1", "W2", "MOB-1", "HH を直す"),
        item("S1", "W1", "HH-3", "無関係", []),
      ],
      projects: [],
    };
    // 「hh」: いまの ID が含むもの(HH-2・HH-3・HH-12・ro の HH-1)が先、タイトルだけのもの(MOB-1・MOB-2)が後。
    expect(ids(matchItems(index, stores, "hh", now))).toEqual([
      "HH-2",
      "HH-3",
      "HH-12",
      "HH-1",
      "MOB-1",
      "MOB-2",
    ]);
  });

  test("ID は小文字で比べる: 「hh-1」で HH-1・HH-12 が出る。前後の空白は除く", () => {
    const index = {
      items: [
        item("S1", "W1", "HH-12", "a"),
        item("S1", "W1", "HH-1", "b"),
        item("S1", "W1", "HH-2", "c"),
      ],
      projects: [],
    };
    expect(ids(matchItems(index, stores, "  hh-1 ", now))).toEqual(["HH-1", "HH-12"]);
  });

  test("入力が空なら何も一致しない", () => {
    const index = { items: [item("S1", "W1", "HH-1", "a")], projects: [] };
    expect(matchItems(index, stores, "  ", now)).toEqual([]);
  });

  test("ヒントはプロジェクト名。旧 ID で一致したら「{プロジェクト名}・旧 ID・{日付} に移動」、旧い接頭辞なら「{プロジェクト名}・旧 ID」", () => {
    const moved = item("S1", "W2", "MOB-5", "a", [
      { id: "HH-40", at: "2026-03-02T10:00:00.000+09:00" },
    ]);
    const renamed = item("S1", "W2", "MOB-6", "b", [{ id: "OLD-6", at: null }]);
    const lastYear = item("S1", "W2", "MOB-7", "c", [
      { id: "HH-41", at: "2025-12-31T10:00:00.000+09:00" },
    ]);
    const index = { items: [moved, renamed, lastYear], projects: [] };
    expect(matchItems(index, stores, "hh-40", now)).toEqual([
      { c: moved, hint: "プロジェクト W2・旧 ID・3/2 に移動" },
    ]);
    expect(matchItems(index, stores, "old-6", now)).toEqual([
      { c: renamed, hint: "プロジェクト W2・旧 ID" },
    ]);
    expect(matchItems(index, stores, "hh-41", now)[0]?.hint).toBe(
      "プロジェクト W2・旧 ID・2025/12/31 に移動",
    );
  });

  test("いまの ID でも一致したら旧 ID を添えない。複数の旧 ID が一致したら新しい日時のもの", () => {
    const both = item("S1", "W1", "HH-7", "a", [{ id: "HH-70", at: "2026-01-01T00:00:00.000Z" }]);
    const many = item("S1", "W2", "MOB-8", "b", [
      { id: "XX-8", at: null },
      { id: "XX-80", at: "2026-02-01T12:00:00.000+09:00" },
      { id: "XX-81", at: "2026-04-05T12:00:00.000+09:00" },
    ]);
    const index = { items: [both, many], projects: [] };
    expect(matchItems(index, stores, "hh-7", now)).toEqual([{ c: both, hint: "プロジェクト W1" }]);
    expect(matchItems(index, stores, "xx-8", now)).toEqual([
      { c: many, hint: "プロジェクト W2・旧 ID・4/5 に移動" },
    ]);
  });

  test("WEB-3 で、いまの WEB-3 と旧い接頭辞 WEB の 3 番(いまの MOB-3)の両方が出る", () => {
    const webStores: StoreNode[] = [
      {
        id: "S1",
        name: "work",
        color: "teal",
        readOnly: false,
        workspaces: [
          ws("S1", "W1", "Web", "WEB", ["P1"]),
          ws("S1", "W2", "Mobile", "MOB", ["P3", "P5"]),
        ],
      },
    ];
    const current = item("S1", "W1", "WEB-3", "a");
    const old = item("S1", "W2", "MOB-3", "b", [{ id: "WEB-3", at: null }]);
    const index = { items: [old, current], projects: [] };
    expect(matchItems(index, webStores, "WEB-3", now)).toEqual([
      { c: current, hint: "プロジェクト W1" },
      { c: old, hint: "プロジェクト W2・旧 ID" },
    ]);
  });
});

describe("matchProjects", () => {
  test("ID の一致が先、タイトルだけの一致が後で、それぞれサイドバーの並び。ヒントはワークスペース名・旧 ID", () => {
    const p1 = project("S1", "W1", "P1", "HH-P1", "検索");
    const p2 = project("S1", "W1", "P2", "HH-P2", "hh の改善");
    const p3 = project("S1", "W2", "P3", "MOB-P1", "アプリ", [
      { id: "HH-P9", at: "2026-06-01T12:00:00.000+09:00" },
    ]);
    const p4 = project("S2", "W3", "P4", "HH-P1", "閲覧");
    const p5 = project("S1", "W2", "P5", "MOB-P2", "その他", [{ id: "HH-P8", at: null }]);
    const index = { items: [], projects: [p4, p3, p2, p1, p5] };
    const got = matchProjects(index, stores, "hh-p", now);
    expect(got.map((x) => x.c.display)).toEqual(["HH-P1", "HH-P2", "MOB-P1", "MOB-P2", "HH-P1"]);
    expect(got.map((x) => x.hint)).toEqual(["Web", "Web", "旧 ID・6/1 に移動", "旧 ID", "Web"]);
    expect(matchProjects(index, stores, "HH", now).map((x) => x.c.title)).toEqual([
      "検索",
      "hh の改善",
      "アプリ",
      "その他",
      "閲覧",
    ]);
    expect(matchProjects(index, stores, "改善", now)).toEqual([{ c: p2, hint: "Web" }]);
    expect(matchProjects(index, stores, "", now)).toEqual([]);
  });
});

describe("matchWorkspaces", () => {
  test("名前か接頭辞が入力を含むもの。サイドバーの並びで、ヒントは Store 名", () => {
    const got = matchWorkspaces(stores, "WE");
    expect(got.map((x) => [x.c.id, x.hint])).toEqual([
      ["W1", "work"],
      ["W3", "ro"],
    ]);
    expect(matchWorkspaces(stores, "mob").map((x) => x.c.id)).toEqual(["W2"]);
    expect(matchWorkspaces(stores, " ")).toEqual([]);
  });
});

describe("byName", () => {
  test("空なら全部、入力があれば名前に含むもの(大文字小文字を区別しない)", () => {
    const list = [
      { label: "新規作成" },
      { label: "設定を開く" },
      { label: "すべての Store を同期" },
    ];
    expect(byName(list, "")).toEqual(list);
    expect(byName(list, " store ")).toEqual([{ label: "すべての Store を同期" }]);
    expect(byName(list, "開く")).toEqual([{ label: "設定を開く" }]);
  });
});
