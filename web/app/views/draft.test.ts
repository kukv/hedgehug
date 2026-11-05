import { describe, expect, it } from "vitest";
import { DEFAULT_KANBAN, DEFAULT_TABLE, type ViewDraft } from "../../core/views";
import { newDraft, sameViewName, summaryOf, treeOrderProblem } from "./draft";

const draft = (over: Partial<ViewDraft> = {}): ViewDraft => ({
  layout: "tree",
  query: "",
  kanban: DEFAULT_KANBAN,
  table: DEFAULT_TABLE,
  ...over,
});

describe("summaryOf", () => {
  it("ツリーは名前だけ", () => {
    expect(summaryOf(draft())).toBe("ツリー");
  });

  it("カンバンは列とスイムレーン", () => {
    expect(summaryOf(draft({ layout: "kanban" }))).toBe(
      "カンバン · 列: ステータス · スイムレーン: なし",
    );
    expect(
      summaryOf(draft({ layout: "kanban", kanban: { columns: "category", swimlane: "assignee" } })),
    ).toBe("カンバン · 列: カテゴリ · スイムレーン: 担当者");
  });

  it("テーブルは列の数とグループ", () => {
    expect(summaryOf(draft({ layout: "table" }))).toBe("テーブル · 列 5 · グループ: なし");
    expect(
      summaryOf(draft({ layout: "table", table: { columns: ["id", "title"], groupBy: "parent" } })),
    ).toBe("テーブル · 列 2 · グループ: 親");
  });
});

describe("treeOrderProblem", () => {
  it("ツリーで sort: か group: があれば、サーバーと同じ理由を返す", () => {
    const why = "ツリーとガントのビューは rank の順に並ぶため、group: と sort: は使えません";
    expect(treeOrderProblem({ layout: "tree", query: "kind:task sort:updated" })).toBe(why);
    expect(treeOrderProblem({ layout: "tree", query: "group:status" })).toBe(why);
  });

  it("ツリーでも、引用符の中の sort: や、ほかの語は問題にしない", () => {
    expect(treeOrderProblem({ layout: "tree", query: 'title:"a sort:b" assignee:@me' })).toBeNull();
    expect(treeOrderProblem({ layout: "tree", query: "" })).toBeNull();
  });

  it("カンバンとテーブルは sort:・group: を持てる", () => {
    expect(treeOrderProblem({ layout: "table", query: "sort:updated" })).toBeNull();
    expect(treeOrderProblem({ layout: "kanban", query: "group:status" })).toBeNull();
  });
});

describe("newDraft", () => {
  it("設定を渡さなければ、レイアウトの既定", () => {
    expect(newDraft({ layout: "table", query: "q" })).toEqual({
      layout: "table",
      query: "q",
      kanban: DEFAULT_KANBAN,
      table: DEFAULT_TABLE,
    });
  });
});

describe("sameViewName", () => {
  it("大文字小文字と Unicode の正規化の違いは同じ名前", () => {
    expect(sameViewName("Weekly", "weekly")).toBe(true);
    expect(sameViewName("が", "か\u3099")).toBe(true);
    expect(sameViewName("weekly", "weekly2")).toBe(false);
  });
});
