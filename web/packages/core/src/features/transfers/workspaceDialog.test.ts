import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { flush } from "../items/fixtures.test-util";
import { setupWorkspace } from "./workspaceDialog.test-util";

describe("workspace transfer section (03「別の Store へ移動」、B5c-15)", () => {
  it("lists the Stores and selects the first", async () => {
    const { transfer, fake, workspace } = await setupWorkspace();
    await transfer.load(workspace);
    expect(fake.calls).toEqual(["workspaceTargets S1-W1"]);
    expect(get(transfer.section)).toEqual({
      workspace,
      options: [
        { value: "S3", label: "team" },
        { value: "S4", label: "far" },
      ],
      value: "S3",
      empty: "",
      disabled: false,
      note: undefined,
      canOpen: true,
    });
    transfer.choose("S4");
    expect(get(transfer.section)?.value).toBe("S4");
    transfer.choose("S9");
    expect(get(transfer.section)?.value).toBe("S4");
  });

  it("shows one of the two sentences without candidates and does not open", async () => {
    const { transfer, fake, workspace } = await setupWorkspace();
    fake.workspaceTargetList = { targets: [], hiddenOnly: false };
    await transfer.load(workspace);
    expect(get(transfer.section)).toMatchObject({
      options: [],
      value: "",
      empty: "移せる Store がありません",
      canOpen: false,
    });
    await transfer.open();
    expect(fake.calls).toEqual(["workspaceTargets S1-W1"]);
    expect(get(transfer.dialog)).toBeNull();
    fake.workspaceTargetList = { targets: [], hiddenOnly: true };
    await transfer.load(workspace);
    expect(get(transfer.section)?.empty).toBe(
      "移せる Store がありません(非表示の Store は、設定の「Store」で表示に戻してください)",
    );
  });

  it("disables the section in a read-only Store and does not open", async () => {
    const { transfer, fake, roWorkspace } = await setupWorkspace();
    await transfer.load(roWorkspace);
    expect(get(transfer.section)).toMatchObject({
      disabled: true,
      note: "閲覧のみの Store です",
      canOpen: false,
    });
    await transfer.open();
    expect(fake.calls).toEqual(["workspaceTargets S2-W1"]);
    expect(get(transfer.dialog)).toBeNull();
  });

  it("drops the candidates of a workspace loaded before the current one", async () => {
    const { transfer, fake, workspace } = await setupWorkspace();
    const h = fake.holdNext();
    const first = transfer.load(workspace);
    await flush();
    fake.workspaceTargetList = { targets: [], hiddenOnly: false };
    await transfer.load({ ...workspace, id: "S1-W2" });
    h.release();
    await first;
    expect(get(transfer.section)).toMatchObject({ workspace: { id: "S1-W2" }, options: [] });
  });

  it("keeps the chosen Store when the same workspace is read again", async () => {
    const { transfer, fake, workspace } = await setupWorkspace();
    await transfer.load(workspace);
    transfer.choose("S4");
    await transfer.load(workspace);
    expect(get(transfer.section)?.value).toBe("S4");
    fake.workspaceTargetList = {
      targets: [{ storeId: "S3", storeName: "team" }],
      hiddenOnly: false,
    };
    await transfer.load(workspace);
    expect(get(transfer.section)?.value).toBe("S3");
  });

  it("shows no section when the candidates cannot be read", async () => {
    const { transfer, fake, workspace } = await setupWorkspace();
    fake.failNext();
    await transfer.load(workspace);
    expect(get(transfer.section)).toBeNull();
  });
});

describe("workspace transfer dialog: opening (03 の確認ダイアログ、B5c-16)", () => {
  it("reads the preview of the selected Store and shows the dialog", async () => {
    const { transfer, fake, workspace } = await setupWorkspace();
    fake.workspacePreviewOf.S4 = {
      source: "work",
      destination: "far",
      projects: 2,
      items: 5,
      deleted: 3,
      people: [
        { login: "suzuki", name: "鈴木" },
        { login: "takahashi", name: "高橋" },
      ],
      remoteNotice: true,
      destinationHasRemote: false,
    };
    await transfer.load(workspace);
    transfer.choose("S4");
    await transfer.open();
    expect(fake.calls.at(-1)).toBe("workspacePreview S1-W1 S4");
    expect(get(transfer.dialog)).toEqual({
      workspace,
      title: "別の Store へ移動",
      lines: [
        "「Web」を work から far へ移動します。",
        "プロジェクト 2 件・アイテム 5 件を移動します。削除済みの 3 件も一緒に移ります。",
        "ワークスペースの外にあるアイテムとの関連は、種類を問わず解除されます。ワークスペースの中のアイテム同士の関連は残ります。",
        "鈴木さん・高橋さんの 2 人が、移動先のメンバーに追加されます",
        "移動先に remote がないため、ほかの人の Store からは見えなくなります。",
      ],
      label: "確認のため、ワークスペースの名前を入力してください",
      ok: "移動",
      error: null,
      busy: false,
    });
  });

  it("toasts a refusal or a failure of the preview and shows no dialog", async () => {
    const { transfer, fake, env, workspace, toasts } = await setupWorkspace();
    await transfer.load(workspace);
    fake.refuseNext("team: この Store には同じ接頭辞のワークスペースがあります");
    await transfer.open();
    fake.failNext();
    await transfer.open();
    expect(get(transfer.dialog)).toBeNull();
    expect(toasts()).toEqual([
      { text: "team: この Store には同じ接頭辞のワークスペースがあります", error: true },
      { text: "移動できませんでした", error: true },
    ]);
    expect(env.reloads).toBe(0);
  });

  it("reloads the catalog when the workspace is gone", async () => {
    const { transfer, fake, env, workspace } = await setupWorkspace();
    await transfer.load(workspace);
    fake.missing.add("S1-W1");
    await transfer.open();
    expect(get(transfer.dialog)).toBeNull();
    expect(env.reloads).toBe(1);
  });

  it("reads the preview once when opened again while reading", async () => {
    const { transfer, fake, workspace } = await setupWorkspace();
    await transfer.load(workspace);
    const h = fake.holdNext();
    const opening = transfer.open();
    await flush();
    await transfer.open();
    h.release();
    await opening;
    expect(fake.calls.filter((c) => c.startsWith("workspacePreview"))).toHaveLength(1);
    expect(get(transfer.dialog)).not.toBeNull();
  });

  it("drops a preview read before cancel", async () => {
    const { transfer, fake, workspace } = await setupWorkspace();
    await transfer.load(workspace);
    const h = fake.holdNext();
    const opening = transfer.open();
    await flush();
    transfer.cancel();
    h.release();
    await opening;
    expect(get(transfer.dialog)).toBeNull();
  });
});
