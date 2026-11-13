import { describe, expect, it } from "vitest";
import type { PropertyType, WorkspaceProperty, WorkspaceSettings } from "../../model/workspace";
import {
  addableTypes,
  deletePropertyBody,
  keyHint,
  newProperty,
  propertiesHelp,
  propertyKinds,
  propertyRows,
  toggleKind,
  typeLabels,
} from "./properties";

const builtin = (key: string, name: string, type: PropertyType): WorkspaceProperty => ({
  id: key,
  key,
  name,
  type,
  builtin: true,
  kinds: [],
  hidden: false,
  choices: [],
});

function settings(): WorkspaceSettings {
  return {
    properties: [
      builtin("status", "ステータス", "status"),
      builtin("assignee", "担当者", "user"),
      builtin("priority", "優先度", "priority"),
      builtin("start", "開始日", "date"),
      builtin("end", "終了日", "date"),
      builtin("points", "ポイント", "number"),
      builtin("estimate", "見積もり時間", "duration"),
      builtin("systems", "関連システム", "systems"),
    ],
    statuses: [],
    priorities: [],
    systems: [],
    initialStatus: "",
    initialPriority: "",
    categoryDefaults: {},
    workflow: {},
    templates: [],
    templateDefaults: {},
    projectRules: {},
  };
}

// 「環境」(単一選択、Story と Task、選択肢 2 つ)を足した設定
function withSingleChoice(s: WorkspaceSettings): WorkspaceSettings {
  return {
    ...s,
    properties: [
      ...s.properties,
      {
        id: "pp-env",
        key: "env",
        name: "環境",
        type: "single",
        builtin: false,
        kinds: ["Story", "Task"],
        hidden: false,
        choices: [
          { id: "c1", name: "本番", color: "gray" },
          { id: "c2", name: "検証", color: "gray" },
        ],
      },
    ],
  };
}

describe("property rows", () => {
  it("lists the eight built-ins with their controls", () => {
    const rows = propertyRows(settings());
    expect(
      rows.map((r) => [r.key, r.type, r.hide ?? "", r.deletable, r.kindsFixed, r.kinds]),
    ).toEqual([
      ["status", "ステータス", "", false, true, "all"],
      ["assignee", "ユーザー", "非表示", false, false, "all"],
      ["priority", "優先度", "非表示", false, false, "all"],
      ["start", "日付", "非表示", false, false, "all"],
      ["end", "日付", "非表示", false, false, "all"],
      ["points", "数値", "非表示", false, false, "all"],
      ["estimate", "時間", "非表示", false, false, "all"],
      ["systems", "関連システム", "非表示", false, false, "all"],
    ]);
  });

  it("shows choices, kinds and a trash for an added property", () => {
    const row = propertyRows(withSingleChoice(settings())).at(-1);
    expect([row?.type, row?.choices, row?.kinds, row?.deletable]).toEqual([
      "単一選択",
      2,
      ["Story", "Task"],
      true,
    ]);
  });

  it("treats all five kinds as all, and marks hidden and the ends", () => {
    const s = settings();
    const second = s.properties[1];
    if (second) {
      second.hidden = true;
      second.kinds = [...propertyKinds];
    }
    const rows = propertyRows(s);
    expect([rows[1]?.hide, rows[1]?.hidden, rows[1]?.kinds]).toEqual(["表示", true, "all"]);
    expect([rows[0]?.first, rows[0]?.last, rows.at(-1)?.last]).toEqual([true, false, true]);
  });

  it("writes the delete body of spec 03", () => {
    expect(deletePropertyBody("顧客", 0)).toBe("「顧客」を削除します。");
    expect(deletePropertyBody("顧客", 3)).toBe(
      "「顧客」を削除します。値の入っている 3 件から値が消えます。",
    );
  });

  it("keeps the texts of spec 03", () => {
    expect(propertiesHelp).toBe(
      "組み込みのプロパティは名前の変更や、種類ごとの表示・非表示を設定できます。追加したプロパティは、型・選択肢・キーを決められます。キーは絞り込みに使います(例: customer:サンプル)。",
    );
    expect(keyHint).toBe("英小文字・数字・_。絞り込みで使います");
    expect(addableTypes.map((t) => typeLabels[t])).toEqual([
      "テキスト",
      "数値",
      "日付",
      "単一選択",
      "複数選択",
      "ユーザー",
      "チェック",
      "URL",
    ]);
  });
});

describe("kinds", () => {
  it("toggles one kind, from all and from a subset", () => {
    expect(toggleKind("all", "Bug")).toEqual(["Epic", "Story", "Task", "Step"]);
    expect(toggleKind(["Story"], "Task")).toEqual(["Story", "Task"]);
    expect(toggleKind(["Story", "Task"], "Story")).toEqual(["Task"]);
  });

  it("sends an empty list when the last kind is unchecked, so the server refuses it", () => {
    expect(toggleKind(["Story"], "Story")).toEqual([]);
  });

  it("uses all kinds for a new property when none is chosen", () => {
    expect(newProperty("顧客", "customer", "text", [])).toEqual({
      name: "顧客",
      key: "customer",
      type: "text",
      kinds: [...propertyKinds],
    });
    expect(newProperty("顧客", "customer", "text", ["Task"]).kinds).toEqual(["Task"]);
  });
});
