import { describe, expect, test } from "vitest";
import { get } from "../../reactive/store";
import { flush, gate, item } from "../items/fixtures.test-util";
import { settings } from "../items/settings.test-util";
import { paletteFooter } from "./palette";
import { index, labels, named, pick, setup } from "./palette.test-util";

// パレットの状態(02「コマンドパレット」、A5c-2・A5c-9・A5c-15・A5c-16)。

const commands = [
  "# コマンド",
  "新規作成",
  "受信箱を開く",
  "設定を開く",
  "すべての Store を同期",
  "ワークスペースを追加",
];
const selected = ["ステータスを変更…", "Story を追加", "詳細を開く", "別のワークスペースへ移動…"];

describe("createPalette", () => {
  test("開くと索引と最近開いたものを読み、入力が空なら コマンド・選んでいるアイテム・最近開いたもの を出す", async () => {
    const { palette } = await setup();
    await palette.open();
    expect(labels(palette)).toEqual([
      ...commands,
      "# HH-1 に対して",
      ...selected,
      "# 最近開いたもの",
      "HH-2 画面",
    ]);
    const s = get(palette.state);
    // S2 にも HH-1 があるので、見出しに S1 の色の点を付ける(A5c-9)。
    expect(s?.selectedHead).toEqual({ text: "HH-1 に対して", storeColor: "teal" });
    expect(s?.selected).toBe(1);
    expect(s?.mode).toBe("main");
    expect(s?.footer).toBe("↑↓ で選択 ・ Enter で開く ・ Ctrl+Enter で新しいタブ ・ Esc で閉じる");
    expect(paletteFooter(true)).toBe(
      "↑↓ で選択 ・ Enter で開く ・ Cmd+Enter で新しいタブ ・ Esc で閉じる",
    );
  });

  test("同じ ID が別の Store になければ見出しに点を付けない。アイテムを読めなければ群を出さない", async () => {
    const other = await setup({ selected: item("S1", "S1-P1", 2, "Task", null) });
    await other.palette.open();
    expect(get(other.palette.state)?.selectedHead).toEqual({ text: "HH-2 に対して" });
    const failed = await setup({ failItem: true });
    await failed.palette.open();
    expect(labels(failed.palette)).toEqual([...commands, "# 最近開いたもの", "HH-2 画面"]);
    expect(get(failed.palette.state)?.selectedHead).toBeNull();
  });

  test("1 文字ごとに候補を作り直し、選択を先頭の押せる行に戻す。↑↓ は端で止まり、見出しと「ほか N 件」を飛ばす", async () => {
    const { palette, calls } = await setup();
    await palette.open();
    palette.move("down");
    palette.move("down");
    expect(get(palette.state)?.selected).toBe(3);
    palette.input("h");
    // h: アイテム(HH-1・HH-2・ro の HH-1)・プロジェクト・ワークスペース(work と ro の HH)。
    expect(labels(palette)).toEqual([
      "# アイテム",
      "HH-1 認証",
      "HH-2 画面",
      "HH-1 別の認証",
      "# プロジェクト",
      "HH-P1 プロジェクト 1",
      "# ワークスペース",
      "Web",
      "Web",
    ]);
    expect(get(palette.state)?.selected).toBe(1);
    palette.move("up");
    expect(get(palette.state)?.selected).toBe(1);
    palette.move("down");
    palette.move("down");
    palette.move("down");
    expect(get(palette.state)?.selected).toBe(5);
    for (let i = 0; i < 10; i++) palette.move("down");
    expect(get(palette.state)?.selected).toBe(8);
    palette.input("zzz");
    expect(labels(palette)).toEqual(["~ 一致するものはありません"]);
    expect(get(palette.state)?.selected).toBe(-1);
    await palette.choose();
    expect(calls).toEqual([]);
    expect(get(palette.state)?.input).toBe("zzz");
  });

  test("索引を読む前の入力は、読んだあとに絞り直す。読めなかったらアイテム・プロジェクトを出さない", async () => {
    const g = gate();
    const { palette } = await setup({ index: () => g.wait.then(() => index) });
    const opened = palette.open();
    palette.input("認証");
    expect(labels(palette)).toEqual(["~ 一致するものはありません"]);
    g.release();
    await opened;
    expect(labels(palette)).toEqual(["# アイテム", "HH-1 認証", "HH-1 別の認証"]);
    expect(get(palette.state)?.selected).toBe(1);

    const failed = await setup({ index: () => Promise.reject(new Error("offline")) });
    await failed.palette.open();
    failed.palette.input("hh");
    expect(labels(failed.palette)).toEqual(["# ワークスペース", "Web", "Web"]);
  });

  test("読み終わりで行が増えても、選んでいる候補を保つ", async () => {
    const g = gate();
    const { palette } = await setup({ recent: () => g.wait.then(() => []) });
    const opened = palette.open();
    await flush();
    palette.move("down");
    expect(get(palette.state)?.selected).toBe(2);
    g.release();
    await opened;
    expect(get(palette.state)?.selected).toBe(2);
  });

  test("ステータスを変更…: 一覧に切り替え、遷移で移れない項目は選べず理由をヒントに出す。選ぶと setStatus を呼んで閉じる", async () => {
    const { palette, calls } = await setup({
      settings: { ...settings, workflow: { st1: ["st2"] } },
    });
    await palette.open();
    palette.input("ステータス");
    await pick(palette, "ステータスを変更…");
    const s = get(palette.state);
    expect(s?.input).toBe("");
    expect(s?.mode).toEqual({ status: expect.objectContaining({ itemId: "S1-I1" }) });
    expect(labels(palette)).toEqual([
      "# HH-1 のステータス",
      "Todo",
      "InProgress",
      "x Done",
      "x Cancel",
    ]);
    const reason = "ワークフローの制約: Todo から Done には変更できません(InProgress のみ)";
    const done = s?.rows[3];
    expect(done?.kind === "candidate" && [done.c.hint, done.c.reason]).toEqual([reason, reason]);
    const todo = s?.rows[1];
    expect(todo?.kind === "candidate" && [todo.c.checked, todo.c.hint]).toEqual([true, "未着手"]);
    palette.input("prog");
    expect(labels(palette)).toEqual(["# HH-1 のステータス", "InProgress"]);
    palette.input("");
    palette.move("down");
    palette.move("down");
    expect(get(palette.state)?.selected).toBe(2);
    await pick(palette, "Done");
    expect(named(calls, "setStatus")).toEqual([]);
    await pick(palette, "InProgress");
    expect(named(calls, "setStatus").map((c) => c.args)).toEqual([["st2"]]);
    expect(get(palette.state)).toBeNull();
  });

  test("ステータスの一覧で、いまのステータスを選び直したら保存せずに閉じる(ステータスのメニューと同じ)", async () => {
    const { palette, calls } = await setup();
    await palette.open();
    await pick(palette, "ステータスを変更…");
    await pick(palette, "Todo");
    expect(named(calls, "setStatus")).toEqual([]);
    expect(get(palette.state)).toBeNull();
  });

  test("ステータスの設定を読めなければ一覧に切り替えない。閉じて開き直したあとに読み終わっても、いまのパレットを変えない", async () => {
    const failed = await setup({ settingsRead: () => Promise.reject(new Error("offline")) });
    await failed.palette.open();
    const before = labels(failed.palette);
    await pick(failed.palette, "ステータスを変更…");
    expect(get(failed.palette.state)?.mode).toBe("main");
    expect(labels(failed.palette)).toEqual(before);

    const g = gate();
    const { palette } = await setup({ settingsRead: () => g.wait });
    await palette.open();
    const switching = pick(palette, "ステータスを変更…");
    palette.close();
    await palette.open();
    palette.move("down");
    g.release();
    await switching;
    expect(get(palette.state)?.mode).toBe("main");
    expect(get(palette.state)?.selected).toBe(2);
  });

  test("ルートが変わったら閉じる", async () => {
    const { palette, route } = await setup();
    await palette.open();
    expect(get(palette.state)).not.toBeNull();
    route.set({ name: "settings", tab: "accounts" });
    expect(get(palette.state)).toBeNull();
  });

  test("閉じたあとに読み終わっても開き直さない", async () => {
    const g = gate();
    const { palette } = await setup({ index: () => g.wait.then(() => index) });
    const opened = palette.open();
    palette.close();
    g.release();
    await opened;
    expect(get(palette.state)).toBeNull();
  });
});
