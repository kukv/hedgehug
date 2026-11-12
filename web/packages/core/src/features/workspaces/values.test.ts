import { describe, expect, it } from "vitest";
import type { Category, WorkspaceSettings, WorkspaceValue } from "../../model/workspace";
import {
  categoryLabels,
  colors,
  deleteDialog,
  deleteRefusal,
  statusHelp,
  valueListLabel,
  valueRows,
  valuesHelp,
} from "./values";

const ids = {
  todo: "st-todo",
  doing: "st-doing",
  done: "st-done",
  pending: "st-pending",
  cancel: "st-cancel",
  web: "sy-web",
  env: "pp-env",
  prod: "ch-prod",
  stage: "ch-stage",
  high: "pr-high",
  low: "pr-low",
};

const v = (id: string, name: string, category?: Category): WorkspaceValue =>
  category ? { id, name, color: "gray", category } : { id, name, color: "gray" };

function settings(): WorkspaceSettings {
  return {
    properties: [],
    statuses: [
      v(ids.todo, "Todo", "todo"),
      v(ids.doing, "InProgress", "doing"),
      v(ids.done, "Done", "done"),
      v(ids.pending, "Pending", "todo"),
      v(ids.cancel, "Cancel", "canceled"),
    ],
    priorities: [v(ids.high, "High"), v(ids.low, "Low")],
    systems: [v(ids.web, "Web")],
    initialStatus: ids.todo,
    initialPriority: ids.high,
    categoryDefaults: { todo: ids.todo, doing: ids.doing, done: ids.done, canceled: ids.cancel },
  };
}

// 「環境」(単一選択)に「本番」「検証」を足した設定
function withSingleChoice(
  s: WorkspaceSettings,
  type: "single" | "multi" = "single",
): WorkspaceSettings {
  return {
    ...s,
    properties: [
      {
        id: ids.env,
        key: "env",
        name: "環境",
        type,
        builtin: false,
        kinds: [],
        hidden: false,
        choices: [v(ids.prod, "本番"), v(ids.stage, "検証")],
      },
    ],
  };
}

const usage = (values: Record<string, number>) => ({
  items: 0,
  firstNumbers: [],
  values,
  properties: {},
});

describe("value rows", () => {
  it("marks the initial status and disables it for done and canceled", () => {
    const rows = valueRows(settings(), "statuses", usage({ [ids.todo]: 3 }));
    expect(rows.map((r) => [r.name, r.initial, r.count, r.initialDisabled ?? ""])).toEqual([
      ["Todo", true, 3, ""],
      ["InProgress", false, 0, ""],
      ["Done", false, 0, "完了・中止のステータスは初期ステータスにできません"],
      ["Pending", false, 0, ""],
      ["Cancel", false, 0, "完了・中止のステータスは初期ステータスにできません"],
    ]);
    expect(rows[0]?.first && rows[4]?.last).toBe(true);
    expect(rows[1]?.first || rows[1]?.last).toBe(false);
  });

  it("marks the initial priority and has no category for systems", () => {
    const p = valueRows(settings(), "priorities", usage({}));
    expect(p.map((r) => [r.name, r.initial])).toEqual([
      ["High", true],
      ["Low", false],
    ]);
    const s = valueRows(settings(), "systems", usage({ [ids.web]: 2 }));
    expect(s.map((r) => [r.name, r.initial, r.count, r.category])).toEqual([
      ["Web", false, 2, undefined],
    ]);
  });

  it("reads choices of a property", () => {
    const rows = valueRows(withSingleChoice(settings()), ids.env, usage({ [ids.prod]: 1 }));
    expect(rows.map((r) => [r.name, r.count, r.initial])).toEqual([
      ["本番", 1, false],
      ["検証", 0, false],
    ]);
  });

  it("has the 18 colors of spec 03 in order", () => {
    expect(colors.map((c) => c.label).join("、")).toBe(
      "グレー、青灰、茶、赤、ローズ、桃、赤紫、紫、藍、青、空色、水色、青緑、緑、オリーブ、山吹、黄、橙",
    );
    expect(colors).toHaveLength(18);
  });

  it("has the texts of spec 03", () => {
    expect(valuesHelp).toBe("名前・色・順番を変更できます。このワークスペースだけの値です。");
    expect(statusHelp).toBe(
      "分類は、ボードの列・バックログ・バーンダウン・マイタスクで「未着手・進行中・完了・中止」を見分けるのに使います。このワークスペースだけの値です。",
    );
    expect(Object.values(categoryLabels)).toEqual(["未着手", "進行中", "完了", "中止"]);
  });

  it("names the lists", () => {
    const s = withSingleChoice(settings());
    expect([
      valueListLabel(s, "statuses"),
      valueListLabel(s, "priorities"),
      valueListLabel(s, "systems"),
      valueListLabel(s, ids.env),
    ]).toEqual(["ステータス", "優先度", "関連システム", "「環境」の選択肢"]);
  });
});

describe("delete dialog", () => {
  it("says nobody uses it", () => {
    const d = deleteDialog(settings(), "systems", ids.web, usage({}));
    expect([d.title, d.body, d.replacements]).toEqual([
      "「Web」を削除",
      "使っているアイテムはありません。",
      undefined,
    ]);
  });

  it("asks for a replacement of a single value and defaults to the initial status", () => {
    const d = deleteDialog(settings(), "statuses", ids.pending, usage({ [ids.pending]: 4 }));
    expect(d.body).toBe("4 件がこの値を使っています。代わりの値を選んでください。");
    expect(d.initialReplacement).toBe(ids.todo);
    expect(d.replacements?.map((r) => r.label)).toEqual(["Todo", "InProgress", "Done", "Cancel"]);
    expect(d.replaceNote?.("Todo")).toBe("4 件を「Todo」に置き換えます。");
    expect(d.statusNote).toBe(
      "ワークフローの制約と自動化(プロジェクト別のルールを含む)からも削除されます。",
    );
  });

  it("defaults a priority to the initial priority without a note", () => {
    const d = deleteDialog(settings(), "priorities", ids.low, usage({ [ids.low]: 1 }));
    expect(d.initialReplacement).toBe(ids.high);
    expect(d.replacements?.map((r) => r.label)).toEqual(["High"]);
    expect(d.replaceNote).toBeUndefined();
    expect(d.statusNote).toBeUndefined();
  });

  it("shows the status note even when unused", () => {
    const d = deleteDialog(settings(), "statuses", ids.pending, usage({}));
    expect(d.body).toBe("使っているアイテムはありません。");
    expect(d.statusNote).toBeDefined();
    expect(d.replaceNote).toBeUndefined();
  });

  it("removes a multiple value", () => {
    const d = deleteDialog(settings(), "systems", ids.web, usage({ [ids.web]: 2 }));
    expect(d.body).toBe("2 件がこの値を使っています。それらからこの値が削除されます。");
    expect(d.replacements).toBeUndefined();
  });

  it("offers なし for a single choice", () => {
    const d = deleteDialog(
      withSingleChoice(settings()),
      ids.env,
      ids.prod,
      usage({ [ids.prod]: 1 }),
    );
    expect(d.title).toBe("「本番」を削除");
    expect(d.replacements?.map((r) => r.label)).toEqual(["なし", "検証"]);
    expect(d.initialReplacement).toBe("");
    expect(d.statusNote).toBeUndefined();
  });

  it("removes a multi choice without replacement", () => {
    const s = withSingleChoice(settings(), "multi");
    const d = deleteDialog(s, ids.env, ids.prod, usage({ [ids.prod]: 1 }));
    expect(d.replacements).toBeUndefined();
  });
});

describe("delete refusal", () => {
  it("merges the initial and default texts into one sentence for a status that is both", () => {
    expect(deleteRefusal(settings(), "statuses", ids.todo)).toBe(
      "「Todo」は初期ステータスで、未着手の既定のステータスでもあります。先に別のステータスを、初期ステータスと未着手の既定にしてください",
    );
  });

  it("refuses the initial status alone", () => {
    const s = { ...settings(), initialStatus: ids.pending };
    expect(deleteRefusal(s, "statuses", ids.pending)).toBe(
      "初期ステータスは削除できません。先に別のステータスを初期ステータスにしてください",
    );
  });

  it("refuses the default status of a category alone", () => {
    expect(deleteRefusal(settings(), "statuses", ids.doing)).toBe(
      "「InProgress」は進行中の既定のステータスです。先に別のステータスを既定にしてください",
    );
  });

  it("refuses the initial priority", () => {
    expect(deleteRefusal(settings(), "priorities", ids.high)).toBe(
      "初期優先度は削除できません。先に別の優先度を初期優先度にしてください",
    );
  });

  it("does not refuse other values", () => {
    expect(deleteRefusal(settings(), "statuses", ids.pending)).toBeNull();
    expect(deleteRefusal(settings(), "priorities", ids.low)).toBeNull();
    expect(deleteRefusal(settings(), "systems", ids.web)).toBeNull();
    expect(deleteRefusal(withSingleChoice(settings()), ids.env, ids.prod)).toBeNull();
  });
});
