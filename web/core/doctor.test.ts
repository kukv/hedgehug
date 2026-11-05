import { describe, expect, it } from "vitest";
import { type DoctorReport, fixedText, groupFindings, parsePurgeDays, severityOf } from "./doctor";

type Finding = NonNullable<DoctorReport["findings"]>[number];

const f = (over: Partial<Finding> & { check: string }): Finding => ({
  msg: "m",
  severity: "error",
  where: "w",
  ...over,
});

describe("the name of a check", () => {
  it.each([
    ["schema", "スキーマ"],
    ["template", "テンプレート"],
    ["prop_name", "プロパティ名"],
    ["override", "プロジェクトの設定"],
    ["automation", "自動化"],
    ["task", "タスク"],
    ["file", "ファイル"],
    ["view", "ビュー"],
    ["worktree", "作業ツリー"],
    ["merge_driver", "merge driver"],
    ["inbox", "通知"],
    ["git", "git"],
    ["cross_store_ref", "Store をまたぐ参照"],
    ["duplicate_across_stores", "Store をまたぐ重複"],
    ["forward_target", "移動先"],
    ["unexpected_force_push", "想定外の force push"],
    ["workspace", "ワークスペース"],
  ])("%s", (check, name) => {
    expect(groupFindings([f({ check })])[0]?.name).toBe(name);
  });

  it("an unknown check is written as it is", () => {
    expect(groupFindings([f({ check: "brand_new" })])[0]?.name).toBe("brand_new");
  });
});

describe("groupFindings", () => {
  it("groups by check in the order the server sends them", () => {
    const groups = groupFindings([
      f({ check: "schema", msg: "a" }),
      f({ check: "task", msg: "b" }),
      f({ check: "task", msg: "c" }),
      f({ check: "zzz", msg: "d" }),
    ]);
    expect(groups.map((g) => [g.check, g.name, g.items.map((i) => i.msg)])).toEqual([
      ["schema", "スキーマ", ["a"]],
      ["task", "タスク", ["b", "c"]],
      ["zzz", "zzz", ["d"]],
    ]);
  });

  it("a check that comes again later joins its group", () => {
    const groups = groupFindings([
      f({ check: "task" }),
      f({ check: "file" }),
      f({ check: "task" }),
    ]);
    expect(groups.map((g) => [g.check, g.items.length])).toEqual([
      ["task", 2],
      ["file", 1],
    ]);
  });

  it("no findings, no groups", () => {
    expect(groupFindings([])).toEqual([]);
  });
});

describe("severityOf", () => {
  it("error, warning and info; an empty or unknown one is an error", () => {
    expect(severityOf(f({ check: "x", severity: "warning" }))).toBe("warning");
    expect(severityOf(f({ check: "x", severity: "info" }))).toBe("info");
    expect(severityOf(f({ check: "x", severity: "error" }))).toBe("error");
    expect(severityOf(f({ check: "x", severity: "" }))).toBe("error");
    expect(severityOf(f({ check: "x", severity: "odd" }))).toBe("error");
  });
});

describe("fixedText", () => {
  it("tells the kinds that doctor --fix repairs", () => {
    expect(fixedText("id_collision", { old: "A-1", new: "A-2" })).toBe(
      "ID が重なっていたため、A-1 から A-2 に振り直しました",
    );
    expect(fixedText("id_renumbered", "A-1")).toBe("ID を振り直しました（元の ID: A-1）");
    expect(fixedText("id_renumbered", null)).toBe("ID を振り直しました");
    expect(fixedText("id_prefix", null)).toBe(
      "ID の接頭辞がワークスペースと違っていたため、振り直しました",
    );
    expect(fixedText("workspace_mismatch", "01OLD")).toBe(
      "親と違うワークスペースにあったため、親のワークスペースに合わせました",
    );
    expect(fixedText("workspace_revived", null)).toBe(
      "削除されていたワークスペースを復活させました",
    );
    expect(fixedText("parent_deleted", { from: "01P", to: null })).toBe(
      "親が削除されたため、ルートに移しました",
    );
  });

  it("a kind it does not know is written as it is", () => {
    expect(fixedText("brand_new", null)).toBe("brand_new");
  });
});

describe("parsePurgeDays", () => {
  it("reads a whole number of days, 0 included", () => {
    expect(parsePurgeDays("90")).toBe(90);
    expect(parsePurgeDays(" 0 ")).toBe(0);
    expect(parsePurgeDays("９０")).toBe(90); // full-width digits, as an IME writes them
    expect(parsePurgeDays("36500")).toBe(36500);
  });

  it.each(["", " ", "-1", "1.5", "1e2", "abc", "36501", "+3"])("%j is not a period", (s) => {
    expect(parsePurgeDays(s)).toBeNull();
  });
});
