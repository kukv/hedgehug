import { describe, expect, test } from "vitest";
import { labels, named, setup } from "./palette.test-util";

// コマンド「検索を開く」(02「コマンドパレット」、04 の 359 行、E3a-9)。

describe("コマンド「検索を開く」", () => {
  // テーブルで開く入口(E3a-9)。テーブルにするのは移れたあと。Ctrl+Enter の新しいタブでは、このタブのレイアウトを変えない。
  test("search command shows table after moving (Ctrl+Enter opens a new tab)", async () => {
    const { palette, calls } = await setup();
    await palette.open();
    expect(labels(palette).slice(0, 6)).toEqual([
      "# コマンド",
      "新規作成",
      "マイタスクを開く",
      "受信箱を開く",
      "アクティビティを開く",
      "検索を開く",
    ]);
    palette.input("検索を開く");
    await palette.choose(1, true);
    expect(named(calls, "openTab").map((c) => c.args)).toEqual([[{ name: "search" }]]);
    expect(named(calls, "showSearchTable")).toEqual([]);
    await palette.open();
    palette.input("検索を開く");
    await palette.choose(1, false);
    expect(calls.filter((c) => c.name === "go" || c.name === "showSearchTable")).toEqual([
      { name: "go", args: [{ name: "search" }], open: false },
      { name: "showSearchTable", args: [], open: false },
    ]);
  });

  // 「入力を破棄」の確認で断られて移らなかったら、テーブルへの切り替えを残さない。
  test("search command does not show table when the move is refused", async () => {
    const { palette, calls } = await setup({ moved: false });
    await palette.open();
    palette.input("検索を開く");
    await palette.choose(1, false);
    expect(named(calls, "go")).toHaveLength(1);
    expect(named(calls, "showSearchTable")).toEqual([]);
  });
});
