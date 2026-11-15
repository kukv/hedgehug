import { describe, expect, it } from "vitest";
import { searchHelp } from "./help";
import type { SearchRow } from "./types";
import { assigneeText, deletedNote, emptyText, readOnlyBands, treeEmptyText } from "./view";

function row(storeName: string, readOnly: boolean): SearchRow {
  return {
    storeId: `s-${storeName}`,
    storeName,
    storeColor: "blue",
    statusId: "",
    workspaceId: "w1",
    projectId: "p1",
    itemId: `i-${storeName}`,
    display: "HH-1",
    title: "t",
    kind: "task",
    status: null,
    assignee: null,
    priority: null,
    end: "",
    points: null,
    oldIdHit: false,
    readOnly,
    deleted: null,
    start: "",
    estimate: null,
    systems: [],
    sprint: "",
    release: "",
    project: { display: "HH-P1", title: "p" },
    parent: null,
    created: "",
    updated: "",
    unused: [],
    removed: [],
    custom: {},
  };
}

describe("emptyText", () => {
  const filtered = {
    title: "一致するものはありません",
    body: "絞り込みを変えてみてください。",
    create: false,
  };

  it("行があれば出さない", () => {
    expect(emptyText("status:Todo", 3, [])).toBeNull();
  });
  it("エラーがあれば、絞り込み中の文", () => {
    expect(emptyText("status:", 0, ["「status:」に値がありません"])).toEqual(filtered);
  });
  it("0 件で絞り込みがあれば、絞り込み中の文", () => {
    expect(emptyText("status:Todo", 0, [])).toEqual(filtered);
  });
  it("絞り込みが空(空白だけを含む)で 0 件なら「まだ何もありません」と作成", () => {
    const empty = { title: "まだ何もありません", body: null, create: true };
    expect(emptyText("", 0, [])).toEqual(empty);
    expect(emptyText("  \t ", 0, [])).toEqual(empty);
  });
});

describe("readOnlyBands", () => {
  it("閲覧のみの Store ごとに 1 本、最初に出た順", () => {
    const rows = [row("main", false), row("team", true), row("docs", true), row("team", true)];
    expect(readOnlyBands(rows)).toEqual([
      "team は閲覧のみの Store です。閲覧はできますが、作成・変更・削除はできません。",
      "docs は閲覧のみの Store です。閲覧はできますが、作成・変更・削除はできません。",
    ]);
  });
  it("閲覧のみの行がなければ空", () => {
    expect(readOnlyBands([row("main", false)])).toEqual([]);
  });
});

describe("deletedNote", () => {
  it("残りの日数か、過ぎたこと", () => {
    expect(deletedNote(16)).toBe("あと 16 日で完全に削除");
    expect(deletedNote(1)).toBe("あと 1 日で完全に削除");
    // 08: 保持日数を過ぎて残っている行は「あと 0 日」ではなく「保持日数を過ぎています」。
    expect(deletedNote(0)).toBe("保持日数を過ぎています");
    expect(deletedNote(-1)).toBe("保持日数を過ぎています");
  });
});

describe("assigneeText", () => {
  it("表示名、メンバーにいなければ「不明なユーザー」、担当者なしは null", () => {
    expect(assigneeText({ login: "tanaka", name: "田中" })).toBe("田中");
    expect(assigneeText({ login: "ghost", name: null })).toBe("不明なユーザー");
    expect(assigneeText(null)).toBeNull();
  });
});

describe("searchHelp", () => {
  it("説明文は 04 の 320 行のとおり", () => {
    expect(searchHelp.lead).toBe(
      "語をスペースで区切ると、すべてに一致するものだけが残ります。`-` を前に付けると否定、`,` で区切るとどれか 1 つに一致します。",
    );
  });
  it("表は 04 の 326〜352 行と同じ順・同じ文", () => {
    expect(searchHelp.rows).toEqual([
      ["`status:InProgress`", "ステータスの名前"],
      ["`category:done`", "ステータスの分類(todo / doing / done / canceled)"],
      ["`kind:story,bug`", "種類(epic / story / task / bug / step)"],
      ["`assignee:@me` / `assignee:none`", "担当者(ログイン名、@me、none)"],
      ["`watching:@me`", "ウォッチしている人(ログイン名、@me、none)"],
      [
        "`mentions:@me`",
        "説明やコメントで呼ばれている人(ログイン名、@me、none)。いまの本文から探す",
      ],
      ["`system:API`", "関連システムの名前"],
      ["`priority:high`", "優先度の名前"],
      ["`sprint:current` / `sprint:none`", "スプリント(current / next / none / 名前)"],
      ["`release:v2.0.0`", "リリースの名前"],
      ["`project:HH-P1`", "プロジェクトの ID(日本語で打つと候補が出る)"],
      ["`parent:HH-1`", "祖先の ID(親、親の親も含む)"],
      [
        "`blocked_by:HH-1` / `relates_to:HH-1`",
        "関連(blocks・blocked_by・causes・caused_by・duplicates・duplicated_by・relates_to・clones・cloned_by)。ブロック中かどうかは `is:blocked`(終わっていないブロッカーがあるもの)で書く",
      ],
      ["`workspace:HH` / `store:team-b`", "ワークスペースの名前か接頭辞、Store の名前"],
      ["`is:blocked` / `is:overdue`", "状態(ブロック中・期限切れ)"],
      ["`is:invalid`", "診断で直せるデータの問題があるもの(理由と直し方は設定の「診断と修復」)"],
      ["`is:deleted`", "削除済みのアイテム(テーブルで表示。復元は設定の「削除済み」)"],
      ["`is:overwritten`", "このマシンの受信箱に、未読の「上書き」の通知があるもの"],
      [
        "`points>=3` / `end<today` / `start>-7d`",
        "数値と日付の比較(today、+3d、-7d も使用できます)",
      ],
      ["`estimate<=2`", "見積もり時間(時間)の比較"],
      ["`updated>-7d` / `created:today`", "変更した日、作成した日の比較"],
      ["`sort:-updated`", "最近変わった順(`created` も使える)"],
      ["`has:end` / `has:pr`", "値があるもの"],
      ["`customer:サンプル`", "ワークスペースで追加したプロパティのキー"],
      ["`sort:-priority`", "並び順(- で降順。テーブルとカンバンで使われる)"],
      ["`group:kind`", "まとめ方(テーブルとカンバン。項目はテーブルの「グループ」と同じ)"],
      [
        '`ガント` / `"ガント 表"`',
        "語だけを書くと、ID・タイトル・説明に含む語。空白を含むときは引用符で囲む",
      ],
    ]);
  });
});

describe("treeEmptyText", () => {
  const noMatch = {
    title: "一致するものはありません",
    body: "絞り込みを変えてみてください。",
    create: false,
  };

  it("行があれば出さない", () => {
    expect(treeEmptyText({ hasProjects: true, rows: 2 }, [])).toBeNull();
  });
  it("表示中の Store にプロジェクトがなければ「プロジェクトはまだありません」(作成のボタンは出さない)", () => {
    expect(treeEmptyText({ hasProjects: false, rows: 0 }, [])).toEqual({
      title: "プロジェクトはまだありません",
      body: "ワークスペースにプロジェクトを作ると、ここに Epic・Story・Task・Bug・Step の木が並びます。",
      create: false,
    });
  });
  it("行が 0 件なら、絞り込み中の文", () => {
    expect(treeEmptyText({ hasProjects: true, rows: 0 }, [])).toEqual(noMatch);
  });
  it("書き方に誤りがあれば、絞り込み中の文", () => {
    expect(treeEmptyText({ hasProjects: true, rows: 3 }, ["「status:」に値がありません"])).toEqual(
      noMatch,
    );
  });
});
