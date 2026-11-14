import { describe, expect, it } from "vitest";
import type { BulkChangeResult, BulkChoices } from "../../model/bulkChange";
import {
  bulkFields,
  changeButton,
  confirmLines,
  explanation,
  inputOf,
  summaryText,
  valueOptions,
} from "./bulkChangeForm";

const choices: BulkChoices = {
  kinds: ["Story", "Task"],
  statuses: ["Todo", "Done"],
  priorities: ["High", "Low"],
  sprints: ["Sprint 1"],
  releases: ["v1"],
  systems: ["Web", "API"],
  members: [{ login: "kukv", name: "Koki" }],
};
const result = (valueText: string): BulkChangeResult => ({
  changed: 3,
  kept: [],
  failed: [],
  offBacklog: { done: 0, canceled: 0 },
  valueText,
});

describe("fields and buttons", () => {
  it("11 fields in order", () => {
    expect(bulkFields.map((f) => f.label)).toEqual([
      "ステータス",
      "担当者",
      "優先度",
      "スプリント",
      "リリース",
      "関連システムを追加",
      "関連システムを削除",
      "ポイント",
      "見積もり時間",
      "開始日",
      "終了日",
    ]);
    expect(bulkFields.map((f) => f.id)[5]).toBe("systemsAdd");
  });
  it("button", () => {
    expect(changeButton("status")).toBe("ステータスを変更");
    expect(changeButton("systemsAdd")).toBe("関連システムを追加");
    expect(changeButton("systemsRemove")).toBe("関連システムを削除");
  });
});

describe("value options", () => {
  it("status ends with categories", () => {
    expect(valueOptions("status", choices)).toEqual([
      { value: "Todo", label: "Todo" },
      { value: "Done", label: "Done" },
      { value: "category:todo", label: "未着手", group: "分類で変更" },
      { value: "category:doing", label: "進行中", group: "分類で変更" },
      { value: "category:done", label: "完了", group: "分類で変更" },
      { value: "category:canceled", label: "中止", group: "分類で変更" },
    ]);
  });
  it("blank first entries", () => {
    expect(valueOptions("sprint", choices)).toEqual([
      { value: "", label: "バックログ" },
      { value: "Sprint 1", label: "Sprint 1" },
    ]);
    expect(valueOptions("release", choices)[0]).toEqual({ value: "", label: "なし" });
    expect(valueOptions("assignee", choices)).toEqual([
      { value: "", label: "未割り当て" },
      { value: "kukv", label: "Koki" },
    ]);
  });
  it("priority has no blank; typed fields have no options", () => {
    expect(valueOptions("priority", choices).map((o) => o.value)).toEqual(["High", "Low"]);
    expect(valueOptions("systemsAdd", choices).map((o) => o.label)).toEqual(["Web", "API"]);
    expect(valueOptions("points", choices)).toEqual([]);
  });
  it("inputOf", () => {
    expect(inputOf("status", "category:done")).toEqual({
      field: "status",
      value: "",
      values: [],
      category: "done",
    });
    expect(inputOf("status", "Done")).toEqual({
      field: "status",
      value: "Done",
      values: [],
      category: "",
    });
  });
});

describe("explanation", () => {
  it("status has one line", () => {
    expect(explanation("status")).toEqual([
      "値は名前で合わせます。ワークスペースに同じ名前の値がないアイテム、ワークフローの制約に合わないアイテム、プロジェクトで使わないステータスにはできないアイテムは変えずに残し、件数を知らせます。",
    ]);
  });
  it("points have a second line", () => {
    expect(explanation("points")[1]).toBe("空にすると値を削除します。");
    expect(explanation("end")).toHaveLength(2);
    expect(explanation("sprint")).toHaveLength(1);
  });
});

describe("summary", () => {
  it("count and kinds", () => {
    expect(summaryText(3, ["Story", "Task"])).toBe("3 件(Story・Task)");
  });
});

describe("confirm lines", () => {
  it("status by name", () => {
    expect(confirmLines(inputOf("status", "Done"), result("Done"))).toEqual([
      "3 件のステータスを Done に変更します。",
    ]);
  });
  it("status by category", () => {
    expect(confirmLines(inputOf("status", "category:done"), result(""))).toEqual([
      "3 件のステータスを 分類「完了」(各ワークスペースの既定のステータス)に変更します。",
    ]);
  });
  it("systems", () => {
    expect(confirmLines(inputOf("systemsAdd", ""), result("Web, API"))).toEqual([
      "3 件に関連システム「Web, API」を追加します。",
    ]);
    expect(confirmLines(inputOf("systemsRemove", ""), result("Web, API"))).toEqual([
      "3 件から関連システム「Web, API」を削除します。",
    ]);
  });
  it("sprint to backlog", () => {
    expect(confirmLines(inputOf("sprint", ""), result("バックログ"))).toEqual([
      "3 件のスプリントを バックログ に変更します。",
    ]);
  });
});
