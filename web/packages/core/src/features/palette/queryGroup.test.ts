import { describe, expect, test } from "vitest";
import { get } from "../../reactive/store";
import { flush, gate } from "../items/fixtures.test-util";
import { found, index, labels, named, noKey, pi, pick, setup } from "./palette.test-util";

// パレットの「このクエリで」の群(02「コマンドパレット」)。

describe("このクエリで(E1-21)", () => {
  test("150ms 待って解析し、キーがありエラーがなければ「検索で開く」を件数のヒントつきで出す。選ぶと閉じて openSearch", async () => {
    const { palette, calls, timer, searched } = await setup({ search: async () => found(3) });
    await palette.open();
    palette.input("kind:bug");
    expect(searched).toEqual([]);
    expect(timer.fire()).toEqual([150]);
    await flush();
    expect(searched).toEqual(["kind:bug"]);
    expect(labels(palette).slice(0, 2)).toEqual(["# このクエリで", "検索で開く"]);
    const row = get(palette.state)?.rows[1];
    expect(row?.kind === "candidate" && row.c.hint).toBe("3 件");
    await pick(palette, "検索で開く");
    expect(named(calls, "openSearch")).toEqual([
      { name: "openSearch", args: ["kind:bug"], open: false },
    ]);
    expect(get(palette.state)).toBeNull();
  });

  test("「検索で開く」の次に「{N} 件をまとめて変更…」を出す。選ぶと閉じて bulkChangeAll", async () => {
    const { palette, calls, timer } = await setup({ search: async () => found(3) });
    await palette.open();
    palette.input("kind:bug");
    timer.fire();
    await flush();
    expect(labels(palette).slice(0, 4)).toEqual([
      "# このクエリで",
      "検索で開く",
      "ビューとして保存…",
      "3 件をまとめて変更…",
    ]);
    await pick(palette, "3 件をまとめて変更…");
    expect(named(calls, "bulkChangeAll")).toEqual([
      { name: "bulkChangeAll", args: ["kind:bug"], open: false },
    ]);
    expect(get(palette.state)).toBeNull();
  });

  test.each([
    ["0 件", () => found(0), "0 件をまとめて変更…", "一致するアイテムがありません"],
    [
      "すべて閲覧のみ",
      () => {
        const r = found(2);
        return { ...r, rows: r.rows.map(() => ({ readOnly: true }) as (typeof r.rows)[number]) };
      },
      "2 件をまとめて変更…",
      "閲覧のみの Store です",
    ],
    [
      "is:deleted",
      () => ({ ...found(2), deleted: true }),
      "2 件をまとめて変更…",
      "削除済みのアイテムは変更できません",
    ],
    [
      "is:deleted の 0 件",
      () => ({ ...found(0), deleted: true }),
      "0 件をまとめて変更…",
      "削除済みのアイテムは変更できません",
    ],
  ])("まとめて変更は %s のとき無効にして理由を添える", async (_n, result, label, reason) => {
    const { palette, timer } = await setup({ search: async () => result() });
    await palette.open();
    palette.input("kind:bug");
    timer.fire();
    await flush();
    expect(labels(palette)).toContain(`x ${label}`);
    const row = get(palette.state)?.rows.find((r) => r.kind === "candidate" && r.c.label === label);
    expect(row?.kind === "candidate" && row.c.reason).toBe(reason);
  });

  test("キーの形の語がなければ、待たずサーバーにも聞かない", async () => {
    const { palette, timer, searched } = await setup({ search: async () => found(1) });
    await palette.open();
    for (const text of ["認証", "HH-1 ログイン", '"status:x"', "- foo", "C++>11"]) {
      palette.input(text);
      expect(timer.count()).toBe(0);
    }
    await flush();
    expect(searched).toEqual([]);
    expect(labels(palette)).not.toContain("# このクエリで");
  });

  test("キーなし・エラーのときは群を出さない", async () => {
    const a = await setup({ search: async () => noKey });
    await a.palette.open();
    a.palette.input("foo:");
    a.timer.fire();
    await flush();
    expect(labels(a.palette)).not.toContain("# このクエリで");
    const b = await setup({
      search: async () => ({ ...found(1), errors: ["「status:」に値がありません"] }),
    });
    await b.palette.open();
    b.palette.input("status:");
    b.timer.fire();
    await flush();
    expect(labels(b.palette)).not.toContain("# このクエリで");
  });

  test("入力が変わったあとに返った古い応答は群にしない。入力のたびに待ち直す", async () => {
    const g = gate();
    const { palette, timer } = await setup({
      search: (q) => (q === "kind:bug" ? g.wait.then(() => found(2)) : Promise.resolve(noKey)),
    });
    await palette.open();
    palette.input("kind:bug");
    timer.fire();
    palette.input("kind:bu");
    expect(timer.count()).toBe(1);
    g.release();
    await flush();
    expect(labels(palette)).not.toContain("# このクエリで");
  });

  test("入力を変えると前の群を消し、閉じると待ちを捨てる", async () => {
    const { palette, timer } = await setup({ search: async () => found(1) });
    await palette.open();
    palette.input("kind:bug");
    timer.fire();
    await flush();
    expect(labels(palette)).toContain("# このクエリで");
    palette.input("kind:bugs");
    expect(labels(palette)).not.toContain("# このクエリで");
    palette.close();
    expect(timer.count()).toBe(0);
  });

  test("閉じて開き直したあとに返った応答は群にしない", async () => {
    const g = gate();
    const { palette, timer } = await setup({ search: () => g.wait.then(() => found(2)) });
    await palette.open();
    palette.input("kind:bug");
    timer.fire();
    palette.close();
    await palette.open();
    g.release();
    await flush();
    expect(labels(palette)).not.toContain("# このクエリで");
  });

  test("アイテムの候補は群があってもなくても変わらない", async () => {
    // キーの形の語でもアイテムに一致するよう、タイトルが「todo:認証」のアイテムを足す。
    const withTodo = async () => ({
      ...index,
      items: [...index.items, pi("S1", 3, "todo:認証")],
    });
    const withGroup = await setup({ search: async () => found(1), index: withTodo });
    await withGroup.palette.open();
    withGroup.palette.input("todo:認証");
    withGroup.timer.fire();
    await flush();
    expect(labels(withGroup.palette)).toContain("# このクエリで");
    expect(labels(withGroup.palette)).toContain("HH-3 todo:認証");
    const without = await setup({ index: withTodo });
    await without.palette.open();
    without.palette.input("todo:認証");
    expect(
      labels(withGroup.palette).filter(
        (l) =>
          l !== "# このクエリで" &&
          l !== "検索で開く" &&
          l !== "ビューとして保存…" &&
          !l.endsWith("件をまとめて変更…"),
      ),
    ).toEqual(labels(without.palette));
  });
});

describe("ビューとして保存…(E3b-16)", () => {
  test("「検索で開く」と「まとめて変更」のあいだに出し、選ぶと閉じて saveQueryAsView", async () => {
    const { palette, calls, timer } = await setup({ search: async () => found(3) });
    await palette.open();
    palette.input("kind:bug");
    timer.fire();
    await flush();
    expect(labels(palette).slice(0, 4)).toEqual([
      "# このクエリで",
      "検索で開く",
      "ビューとして保存…",
      "3 件をまとめて変更…",
    ]);
    await pick(palette, "ビューとして保存…");
    expect(named(calls, "saveQueryAsView")).toEqual([
      { name: "saveQueryAsView", args: ["kind:bug"], open: false },
    ]);
    expect(get(palette.state)).toBeNull();
  });
});
