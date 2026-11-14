import { describe, expect, it } from "vitest";
import type { PaletteItem } from "../../model/palette";
import type { StoreNode } from "../catalog/catalog";
import {
  applyCandidate,
  type CandidateScope,
  type CandidateSources,
  candidateSource,
  filterCandidates,
} from "./candidates";
import { caretTerm } from "./caret";

function store(id: string, color: string, prefix: string, projects: [string, string][]): StoreNode {
  return {
    id,
    name: id,
    color,
    readOnly: false,
    workspaces: [
      {
        id: `w-${id}`,
        storeId: id,
        name: "Web",
        prefix,
        oldPrefixes: [],
        projects: projects.map(([displayId, title], i) => ({
          id: `p-${id}-${i}`,
          storeId: id,
          workspaceId: `w-${id}`,
          number: i + 1,
          rank: "a",
          title,
          owner: "",
          status: "Todo",
          start: "",
          end: "",
          repos: [],
          displayId,
        })),
      },
    ],
  };
}

function item(storeId: string, display: string, title: string): PaletteItem {
  return {
    storeId,
    workspaceId: `w-${storeId}`,
    projectId: "p",
    itemId: `i-${storeId}-${display}`,
    number: 1,
    display,
    title,
    projectTitle: "",
    aliases: [],
  };
}

const src: CandidateSources = {
  stores: [store("s1", "blue", "HH", [["HH-P1", "モバイルアプリ刷新"]])],
  items: [item("s1", "HH-12", "ログイン画面の改修"), item("s1", "HH-13", "検索")],
  members: [
    { storeId: "s1", login: "tanaka", name: "田中 美咲" },
    { storeId: "s2", login: "Tanaka", name: "田中 美咲" },
    { storeId: "s1", login: "suzuki", name: "鈴木" },
  ],
};
const all: CandidateScope = { kind: "all" };
const at = (marked: string, s = src, scope: CandidateScope = all) => {
  const t = caretTerm(marked.replace("|", ""), marked.indexOf("|"));
  if (!t) throw new Error("no term");
  return filterCandidates(t, s, scope);
};

describe("filterCandidates", () => {
  it("finds projects by title and by ID", () => {
    const want = [{ label: "HH-P1 モバイルアプリ刷新", insert: "HH-P1" }];
    expect(at("project:モバ|")).toEqual(want);
    expect(at("project:hh-p|")).toEqual(want);
  });

  it("finds members once per login, ignoring case", () => {
    expect(at("assignee:田中|")).toEqual([{ label: "田中 美咲 @tanaka", insert: "tanaka" }]);
    expect(at("assignee:suz|")).toEqual([{ label: "鈴木 @suzuki", insert: "suzuki" }]);
  });

  it("finds items for parent, epic and the relation keys", () => {
    const want = [{ label: "HH-12 ログイン画面の改修", insert: "HH-12" }];
    expect(at("parent:ログイン|")).toEqual(want);
    expect(at("epic:hh-12|")).toEqual(want);
    const relations = [
      "blocks",
      "blocked_by",
      "causes",
      "caused_by",
      "duplicates",
      "duplicated_by",
      "relates_to",
      "clones",
      "cloned_by",
    ];
    for (const k of relations) expect(at(`${k}:HH-12|`)).toEqual(want);
    expect(at("blocks:HH-1|").map((c) => c.insert)).toEqual(["HH-12", "HH-13"]);
  });

  it("lists a prefix shared by two stores once", () => {
    const two: CandidateSources = {
      stores: [store("s1", "blue", "HH", []), store("s2", "red", "hh", [])],
      items: [],
      members: [],
    };
    expect(at("workspace:web|", two)).toEqual([{ label: "Web HH", insert: "HH" }]);
  });

  it("finds workspaces by name and prefix, labelled 「名前 接頭辞」", () => {
    const want = [{ label: "Web HH", insert: "HH" }];
    expect(at("workspace:we|")).toEqual(want);
    expect(at("workspace:hh|")).toEqual(want);
    expect(at('workspace:"web h|')).toEqual(want);
  });

  it("returns nothing without a match, without a typed value, or for other keys", () => {
    expect(at("project:zzz|")).toEqual([]);
    expect(at("project:|")).toEqual([]);
    expect(at("status:HH|")).toEqual([]);
  });

  it("returns at most 10", () => {
    const many = {
      ...src,
      items: Array.from({ length: 12 }, (_, i) => item("s1", `HH-${100 + i}`, "x")),
    };
    expect(at("parent:HH|", many)).toHaveLength(10);
  });

  it("adds the store colour when the same ID is in another store", () => {
    const two: CandidateSources = {
      stores: [
        store("s1", "blue", "HH", [["HH-P1", "A"]]),
        store("s2", "red", "HH", [["HH-P1", "B"]]),
      ],
      items: [item("s1", "HH-12", "a"), item("s2", "HH-12", "b"), item("s2", "HH-13", "c")],
      members: [],
    };
    expect(at("project:HH-P1|", two)).toEqual([
      { label: "HH-P1 A", insert: "HH-P1", dot: "blue" },
      { label: "HH-P1 B", insert: "HH-P1", dot: "red" },
    ]);
    expect(at("parent:HH-1|", two)).toEqual([
      { label: "HH-12 a", insert: "HH-12", dot: "blue" },
      { label: "HH-12 b", insert: "HH-12", dot: "red" },
      { label: "HH-13 c", insert: "HH-13" },
    ]);
  });

  it("limits to a workspace scope", () => {
    const two: CandidateSources = {
      stores: [
        store("s1", "blue", "HH", [["HH-P1", "A"]]),
        store("s2", "red", "HH", [["HH-P1", "B"]]),
      ],
      items: [item("s1", "HH-12", "a"), item("s2", "HH-12", "b")],
      members: [
        { storeId: "s1", login: "bob", name: "Bob" },
        { storeId: "s2", login: "bea", name: "Bea" },
      ],
    };
    const scope: CandidateScope = { kind: "workspace", storeId: "s2", workspaceId: "w-s2" };
    expect(at("project:HH|", two, scope).map((c) => c.label)).toEqual(["HH-P1 B"]);
    expect(at("parent:HH|", two, scope).map((c) => c.label)).toEqual(["HH-12 b"]);
    expect(at("assignee:b|", two, scope).map((c) => c.insert)).toEqual(["bea"]);
  });
});

describe("applyCandidate", () => {
  it("replaces only the typed part and puts the caret after it", () => {
    const text = "assignee:tanaka,すず kind:bug";
    const term = caretTerm(text, 18);
    if (!term) throw new Error("no term");
    expect(applyCandidate(text, term, { label: "鈴木 @suzuki", insert: "suzuki" })).toEqual({
      text: "assignee:tanaka,suzuki kind:bug",
      caret: 22,
    });
  });

  it("replaces the opening quote too", () => {
    const text = 'parent:"HH';
    const term = caretTerm(text, text.length);
    if (!term) throw new Error("no term");
    expect(applyCandidate(text, term, { label: "", insert: "HH-12" }).text).toBe("parent:HH-12");
  });

  it("also replaces the closing quote right after the caret", () => {
    const text = 'parent:"HH" kind:bug';
    const term = caretTerm(text, 10);
    if (!term) throw new Error("no term");
    expect(applyCandidate(text, term, { label: "", insert: "HH-12" })).toEqual({
      text: "parent:HH-12 kind:bug",
      caret: 12,
    });
  });
});

describe("applyCandidate inside a quoted value", () => {
  const apply = (marked: string) => {
    const text = marked.replace("|", "");
    const term = caretTerm(text, marked.indexOf("|"));
    if (!term) throw new Error("no term");
    return applyCandidate(text, term, { label: "", insert: "HH-12" });
  };

  it("replaces through the closing quote when the caret is inside the quotes", () => {
    expect(apply('parent:"HH|xx" kind:bug')).toEqual({ text: "parent:HH-12 kind:bug", caret: 12 });
    expect(apply('parent:"HH|xx",HH-3')).toEqual({ text: "parent:HH-12,HH-3", caret: 12 });
  });

  it("replaces to the end when the quote is not closed", () => {
    expect(apply('parent:"HH|xx')).toEqual({ text: "parent:HH-12", caret: 12 });
  });

  it("keeps what follows a quote that is already closed before the caret", () => {
    expect(apply('parent:"HH"| kind:"a b"')).toEqual({ text: 'parent:HH-12 kind:"a b"', caret: 12 });
  });
});

describe("candidateSource", () => {
  it("キーの候補が、どの元(読み込みのあるもの)を使うか", () => {
    expect(candidateSource("assignee")).toBe("members");
    for (const key of ["parent", "epic", "blocked_by", "relates_to", "cloned_by"]) {
      expect(candidateSource(key)).toBe("items");
    }
    expect(candidateSource("project")).toBe("catalog");
    expect(candidateSource("workspace")).toBe("catalog");
    expect(candidateSource("status")).toBeNull();
  });
});
