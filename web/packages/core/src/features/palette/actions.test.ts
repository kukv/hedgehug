import { describe, expect, test } from "vitest";
import { get } from "../../reactive/store";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { item } from "../items/fixtures.test-util";
import { epic, labels, named, pick, setup } from "./palette.test-util";

// 候補を選んだときの動き(02「コマンドパレット」の実行の表、A5c-10〜A5c-12・A5c-14・A5c-16)。

const ws = { name: "workspace", store: "work", prefix: "HH" } as const;

describe("パレットの候補の実行", () => {
  test("ダイアログを開く候補は、先に close してから open を呼ぶ", async () => {
    const { palette, calls } = await setup();
    await palette.open();
    await pick(palette, "新規作成");
    expect(get(palette.state)).toBeNull();
    expect(named(calls, "itemDialog.open")).toEqual([
      { name: "itemDialog.open", args: [], open: false },
    ]);
    await palette.open();
    await pick(palette, "ワークスペースを追加");
    expect(named(calls, "workspaceDialog.open")).toEqual([
      { name: "workspaceDialog.open", args: [], open: false },
    ]);
  });

  test("Ctrl+Enter: アイテムは openTab(/s/{Store名}/i/{ID})、新規作成は Enter と同じ", async () => {
    const { palette, calls } = await setup();
    await palette.open();
    palette.input("画面");
    await pick(palette, "HH-2 画面", true);
    expect(get(palette.state)).toBeNull();
    expect(named(calls, "openTab").map((c) => c.args)).toEqual([
      [{ name: "item", store: "work", item: "HH-2" }],
    ]);
    expect(named(calls, "detail.open")).toEqual([]);
    await palette.open();
    await pick(palette, "新規作成", true);
    expect(named(calls, "itemDialog.open")).toHaveLength(1);
    expect(named(calls, "openTab")).toHaveLength(1);
    // Enter のアイテムは詳細パネルを開く。
    await palette.open();
    palette.input("画面");
    await pick(palette, "HH-2 画面");
    expect(named(calls, "detail.open")).toEqual([
      { name: "detail.open", args: ["S1", "S1-I2"], open: false },
    ]);
  });

  test("閲覧のみの Store のアイテムでは、選んでいるアイテムの群は「詳細を開く」だけ", async () => {
    const { palette } = await setup({ selected: item("S2", "S2-P1", 1, "Epic", null) });
    await palette.open();
    const all = labels(palette);
    const at = all.indexOf("# HH-1 に対して");
    expect(all.slice(at, at + 3)).toEqual(["# HH-1 に対して", "詳細を開く", "# 最近開いたもの"]);
  });

  test("詳細パネルで開いているアイテムには「詳細を開く」を出さない", async () => {
    const { palette } = await setup({ target: { storeId: "S1", itemId: epic.id } });
    await palette.open();
    expect(labels(palette)).not.toContain("詳細を開く");
    expect(labels(palette)).toContain("ステータスを変更…");
  });

  test("Step には「{種類} を追加」「別のワークスペースへ移動…」を出さない。Epic は「Story を追加」", async () => {
    const step = await setup({ selected: item("S1", "S1-P1", 3, "Step", "S1-I2") });
    await step.palette.open();
    const all = labels(step.palette);
    const at = all.indexOf("# HH-3 に対して");
    expect(all.slice(at + 1, at + 3)).toEqual(["ステータスを変更…", "詳細を開く"]);
    expect(all.slice(at + 3)[0]).toBe("# 最近開いたもの");

    const { palette, calls } = await setup();
    await palette.open();
    await pick(palette, "Story を追加");
    expect(named(calls, "itemDialog.open")).toEqual([
      {
        name: "itemDialog.open",
        args: [{ workspaceId: "S1-W1", projectId: "S1-P1", kind: "Story", parentId: epic.id }],
        open: false,
      },
    ]);
    await palette.open();
    await pick(palette, "別のワークスペースへ移動…");
    const moved = named(calls, "transferDialog.open");
    expect(moved).toHaveLength(1);
    expect(moved[0]).toMatchObject({ args: [{ id: epic.id, kind: "Epic" }], open: false });
  });

  test("スプリントを作成: ワークスペース・プロジェクトの画面だけで出し、sprints のタブへ go してから ask する", async () => {
    const home = await setup();
    await home.palette.open();
    home.palette.input("作成");
    expect(labels(home.palette)).toEqual([
      "# コマンド",
      "新規作成",
      "# 作成",
      "プロジェクトを作成",
    ]);

    const { palette, calls, planRequest } = await setup({
      here: { workspaceId: "S1-W1", projectId: "S1-P1" },
    });
    await palette.open();
    palette.input("作成");
    expect(labels(palette)).toEqual([
      "# コマンド",
      "新規作成",
      "# 作成",
      "プロジェクトを作成",
      "スプリントを作成",
      "リリースを作成",
    ]);
    await pick(palette, "スプリントを作成");
    expect(named(calls, "go")).toEqual([
      { name: "go", args: [{ ...ws, tab: "sprints" }], open: false },
    ]);
    expect(get(planRequest.request)).toEqual({ workspaceId: "S1-W1", kind: "sprint" });
    await palette.open();
    palette.input("リリース");
    await pick(palette, "リリースを作成");
    expect(named(calls, "go")[1]?.args).toEqual([{ ...ws, tab: "releases" }]);
    expect(get(planRequest.request)).toEqual({ workspaceId: "S1-W1", kind: "release" });
  });

  test("スプリントを作成: 閲覧のみの Store では、タブへ移り「閲覧のみの Store です」", async () => {
    const { palette, calls, toasts, planRequest } = await setup({ here: { workspaceId: "S2-W1" } });
    await palette.open();
    palette.input("スプリント");
    await pick(palette, "スプリントを作成");
    expect(named(calls, "go").map((c) => c.args)).toEqual([
      [{ name: "workspace", store: "ro", prefix: "HH", tab: "sprints" }],
    ]);
    expect(toasts).toEqual(["閲覧のみの Store です"]);
    expect(get(planRequest.request)).toBeNull();
  });

  test("スプリントを作成: 「入力を破棄」の確認で断られて移らなかったら、作成の要求を置かない", async () => {
    const { palette, calls, planRequest } = await setup({
      here: { workspaceId: "S1-W1" },
      moved: false,
    });
    await palette.open();
    palette.input("スプリント");
    await pick(palette, "スプリントを作成");
    expect(named(calls, "go")).toHaveLength(1);
    expect(get(planRequest.request)).toBeNull();
  });

  test("スプリントを作成: 閲覧のみの Store で、確認で断られて移らなかったら「閲覧のみの Store です」を出さない", async () => {
    const { palette, calls, toasts } = await setup({
      here: { workspaceId: "S2-W1" },
      moved: false,
    });
    await palette.open();
    palette.input("スプリント");
    await pick(palette, "スプリントを作成");
    expect(named(calls, "go")).toHaveLength(1);
    expect(toasts).toEqual([]);
  });

  test("プロジェクトを作成: 書き込めるワークスペースがなければ A5c-14 の文", async () => {
    const base = fakeCatalogPort();
    const roOnly = await setup({
      catalogPort: {
        ...base,
        listStores: async () => [{ id: "S2", name: "ro", color: "orange", readOnly: true }],
      },
    });
    await roOnly.palette.open();
    roOnly.palette.input("プロジェクトを");
    await pick(roOnly.palette, "プロジェクトを作成");
    expect(roOnly.toasts).toEqual([
      "プロジェクトを作成できる Store がありません(閲覧のみの Store だけです)",
    ]);
    expect(named(roOnly.calls, "projectDialog.open")).toEqual([]);

    const empty = await setup({ catalogPort: { ...base, listWorkspaces: async () => [] } });
    await empty.palette.open();
    empty.palette.input("プロジェクトを");
    await pick(empty.palette, "プロジェクトを作成");
    expect(empty.toasts).toEqual(["先にワークスペースを作ってください"]);

    const { palette, calls } = await setup({ here: { workspaceId: "S1-W1" } });
    await palette.open();
    palette.input("プロジェクトを");
    await pick(palette, "プロジェクトを作成");
    expect(named(calls, "projectDialog.open")).toEqual([
      { name: "projectDialog.open", args: ["S1-W1"], open: false },
    ]);
  });

  test("すべての Store を同期: sync.syncAll を呼ぶ(remote のない場合の文はサーバーが返す)", async () => {
    const { palette, calls } = await setup();
    await palette.open();
    const row = get(palette.state)?.rows.find(
      (r) => r.kind === "candidate" && r.c.label === "すべての Store を同期",
    );
    expect(row?.kind === "candidate" && row.c.hint).toBe("Shift+Ctrl+S");
    await pick(palette, "すべての Store を同期");
    expect(named(calls, "syncAll")).toEqual([{ name: "syncAll", args: [], open: false }]);
    expect(get(palette.state)).toBeNull();
  });

  test("設定: ワークスペース({名前}) はワークスペース・プロジェクトの画面だけで出し、その設定タブへ go", async () => {
    const home = await setup();
    await home.palette.open();
    home.palette.input("設定");
    expect(labels(home.palette)).toEqual([
      "# コマンド",
      "設定を開く",
      "# 設定",
      "設定: アカウントとメンバー",
    ]);
    await home.palette.choose(1, true);
    expect(named(home.calls, "openTab").map((c) => c.args)).toEqual([
      [{ name: "settings", tab: "accounts" }],
    ]);

    const onWs = await setup({ here: { workspaceId: "S1-W1" } });
    await onWs.palette.open();
    onWs.palette.input("設定:");
    expect(labels(onWs.palette)).toEqual([
      "# 設定",
      "設定: アカウントとメンバー",
      "設定: ワークスペース(Web)",
    ]);

    const { palette, calls } = await setup({ here: { workspaceId: "S1-W1", projectId: "S1-P1" } });
    await palette.open();
    palette.input("設定:");
    expect(labels(palette)).toContain("設定: プロジェクト(プロジェクト 1)");
    await pick(palette, "設定: ワークスペース(Web)");
    expect(named(calls, "go")).toEqual([
      { name: "go", args: [{ ...ws, tab: "settings" }], open: false },
    ]);
    await palette.open();
    palette.input("設定:");
    await pick(palette, "設定: プロジェクト(プロジェクト 1)");
    expect(named(calls, "go")[1]?.args).toEqual([
      { name: "project", store: "work", project: "HH-P1", tab: "settings" },
    ]);
  });
});
