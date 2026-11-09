import { describe, expect, it } from "vitest";
import {
  type DoctorReport,
  findingHint,
  findingText,
  fixedText,
  groupFindings,
  parsePurgeDays,
  severityOf,
} from "./doctor";

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
      "ID が重複していたため、A-1 から A-2 に振り直しました",
    );
    expect(fixedText("id_renumbered", "A-1")).toBe("ID を振り直しました（元の ID: A-1）");
    expect(fixedText("id_renumbered", null)).toBe("ID を振り直しました");
    expect(fixedText("id_prefix", null)).toBe(
      "ID の接頭辞がワークスペースと違っていたため、振り直しました",
    );
    expect(fixedText("workspace_mismatch", "01OLD")).toBe(
      "親と違うワークスペースにあったため、親のワークスペースに合わせました",
    );
    expect(fixedText("workspace_revived", null)).toBe("削除されていたワークスペースを復元しました");
    expect(fixedText("parent_deleted", { from: "01P", to: null })).toBe(
      "親「01P」が削除されたため、ルートに移しました",
    );
    expect(
      fixedText(
        "parent_deleted",
        { from: "01P", to: "01Q", kind_from: "step", kind_to: "task" },
        (u) => (u === "01Q" ? "未分類" : undefined),
      ),
    ).toBe(
      "親「01P」が削除されたため、「未分類」の下に移しました。種類を Step から Task に変えました",
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

describe("findingText and findingHint", () => {
  it("say a finding of the schema in Japanese, with the values it names", () => {
    const x = f({ check: "schema", code: "schema.default_status", args: ["done", "0"], hint: "" });
    expect(findingText(x)).toBe(
      "カテゴリ完了の既定のステータスは 1 つだけにしてください（今は 0 個）",
    );
    expect(findingHint(x)).toBe("");
  });

  it("say an old store format in Japanese, with the command to run", () => {
    const x = f({
      check: "file",
      code: "store.old_format",
      args: ["1", "2"],
      hint: "hh store convert",
    });
    expect(findingText(x)).toBe(
      "Store の形式が古いです（1、最新は 2）。hh store convert で新しい形式にしてください",
    );
    expect(findingHint(x)).toBe("hh store convert");
  });

  it("say the problem of a task by its code, after its field", () => {
    const x = f({
      check: "task",
      code: "task.duplicate_id",
      args: ["id", "id ME-1 is also the id of X"],
      hint: "hedgehug doctor --fix fixes it",
    });
    expect(findingText(x)).toBe("id: ID が重複しています");
    expect(findingHint(x)).toBe(
      "hh doctor --fix で直せます。hh sync でも、push 済みのタスクを残して振り直します",
    );
  });

  it("task.legacy_field: 移せる指摘と、移せない指摘で案内を分ける", () => {
    const base = { check: "task", code: "task.legacy_field", args: ["props.relates", "x"] };
    const movable = f({ ...base, hint: "hedgehug doctor --fix moves it to the field of the task" });
    expect(findingText(movable)).toBe(
      "props.relates: 古い形の関連が props に残っています（関連はタスクの項目です）",
    );
    expect(findingHint(movable)).toBe("hh doctor --fix で関連の項目へ移せます");
    const stuck = f({
      ...base,
      hint: "hedgehug doctor --fix leaves this task as it is, because the field relates has values of its own; decide with hedgehug edit",
    });
    expect(findingHint(stuck)).toBe(
      "移し先に値があるか、値が ULID でないため、--fix では移せません。hh edit で決めてください",
    );
    expect(fixedText("legacy_field", ["relates", "causes"])).toBe(
      "props の relates・causes を関連の項目へ移しました",
    );
  });

  it("schema.unknown_type: task_ref は関連の案内を出す", () => {
    const x = (t: string) => f({ check: "schema", code: "schema.unknown_type", args: [t] });
    expect(findingText(x("task_ref"))).toContain(
      "関連は固定です。hh dep か詳細の関連の欄で付けます",
    );
    expect(findingText(x("foo"))).toBe("「foo」という型はありません");
  });

  it("keep the error of a library as it is, after the Japanese", () => {
    const x = f({
      check: "file",
      where: "tasks/a.md",
      code: "file.undecodable",
      args: ["yaml: bad"],
      hint: "fix",
    });
    expect(findingText(x)).toBe("ファイルを読み込めません: yaml: bad");
    expect(findingHint(x)).toBe(
      "手で直してコミットするか、git restore tasks/a.md で戻してください",
    );
  });

  it("show a finding without a known code as the server wrote it", () => {
    const x = f({ check: "git", msg: "something new", hint: "do this", code: "git.new_kind" });
    expect(findingText(x)).toBe("something new");
    expect(findingHint(x)).toBe("do this");
    expect(findingText(f({ check: "git", msg: "no code" }))).toBe("no code");
  });
});
