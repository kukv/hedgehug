import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { flush } from "../items/fixtures.test-util";
import { setupProject } from "./projectDialog.test-util";

const review = {
  status: "st9",
  name: "Review",
  count: 2,
  choices: [
    { id: "st1", name: "Todo" },
    { id: "st2", name: "Doing" },
  ],
  default: "st2",
};
// pending は、ルールだけが使うステータス(0 件)の行。
const pending = {
  status: "st8",
  name: "Pending",
  count: 0,
  choices: [
    { id: "st1", name: "Todo" },
    { id: "st2", name: "Doing" },
  ],
  default: "st1",
};

describe("project transfer section (02「画面の見出しと構成」、A5b-16)", () => {
  it("lists the candidates as「{Store 名} / {ワークスペース名}」and selects the first", async () => {
    const { transfer, fake, project } = await setupProject();
    await transfer.load(project);
    expect(fake.calls).toEqual(["projectTargets S1-P1"]);
    expect(get(transfer.section)).toEqual({
      project,
      options: [
        { value: "S1/S1-W2", label: "work / Mobile" },
        { value: "S3/S3-W1", label: "team / Team" },
      ],
      value: "S1/S1-W2",
      empty: "",
      disabled: false,
      note: undefined,
      canOpen: true,
    });
    transfer.choose("S3/S3-W1");
    expect(get(transfer.section)?.value).toBe("S3/S3-W1");
    transfer.choose("S9/S9-W1");
    expect(get(transfer.section)?.value).toBe("S3/S3-W1");
  });

  it("shows「移せるワークスペースがありません」without candidates and does not open", async () => {
    const { transfer, fake, project } = await setupProject();
    fake.projectTargetList = [];
    await transfer.load(project);
    expect(get(transfer.section)).toMatchObject({
      options: [],
      value: "",
      empty: "移せるワークスペースがありません",
      canOpen: false,
    });
    await transfer.open();
    expect(fake.calls).toEqual(["projectTargets S1-P1"]);
    expect(get(transfer.dialog)).toBeNull();
  });

  it("disables the section in a read-only Store and does not open", async () => {
    const { transfer, fake, roProject } = await setupProject();
    await transfer.load(roProject);
    expect(get(transfer.section)).toMatchObject({
      disabled: true,
      note: "閲覧のみの Store です",
      canOpen: false,
    });
    await transfer.open();
    expect(fake.calls).toEqual(["projectTargets S2-P1"]);
    expect(get(transfer.dialog)).toBeNull();
  });

  it("drops the candidates of a project loaded before the current one", async () => {
    const { transfer, fake, project } = await setupProject();
    const h = fake.holdNext();
    const first = transfer.load(project);
    await flush();
    fake.projectTargetList = [];
    await transfer.load({ ...project, id: "S1-P2" });
    h.release();
    await first;
    expect(get(transfer.section)).toMatchObject({ project: { id: "S1-P2" }, options: [] });
  });

  it("keeps the chosen destination when the same project is read again", async () => {
    const { transfer, fake, project } = await setupProject();
    await transfer.load(project);
    transfer.choose("S3/S3-W1");
    await transfer.load(project);
    expect(get(transfer.section)?.value).toBe("S3/S3-W1");
    // 選んでいた移動先が候補から消えたら、先頭に戻す。
    fake.projectTargetList = fake.projectTargetList.slice(0, 1);
    await transfer.load(project);
    expect(get(transfer.section)?.value).toBe("S1/S1-W2");
    // 別のプロジェクトは先頭から。
    fake.projectTargetList = [
      { storeId: "S1", storeName: "work", workspaceId: "S1-W2", workspaceName: "Mobile" },
      { storeId: "S3", storeName: "team", workspaceId: "S3-W1", workspaceName: "Team" },
    ];
    transfer.choose("S3/S3-W1");
    await transfer.load({ ...project, id: "S1-P2" });
    expect(get(transfer.section)?.value).toBe("S1/S1-W2");
  });

  it("shows no section when the candidates cannot be read", async () => {
    const { transfer, fake, project } = await setupProject();
    fake.failNext();
    await transfer.load(project);
    expect(get(transfer.section)).toBeNull();
  });
});

describe("project transfer dialog: opening (02 **移動**)", () => {
  it("reads the preview of the selected destination and shows the dialog", async () => {
    const { transfer, fake, project } = await setupProject();
    fake.projectPreviewOf["S3/S3-W1"] = {
      ...fake.defaultProjectPreview,
      items: 3,
      deleted: 1,
      prefix: "TM",
      destination: "team の Team",
      crossStore: true,
      hasRules: true,
      notCarried: [
        { kind: "hidden", name: "Pending", missing: [], count: 0 },
        { kind: "disabled", name: "", missing: [], count: 0 },
        { kind: "template", name: "Bug 報告", missing: [], count: 0 },
      ],
      replacements: [review, pending],
    };
    await transfer.load(project);
    transfer.choose("S3/S3-W1");
    await transfer.open();
    expect(fake.calls.at(-1)).toBe("projectPreview S1-P1 S3/S3-W1");
    expect(get(transfer.dialog)).toEqual({
      project,
      title: "プロジェクトを移動",
      intro:
        "「決済基盤」を Web から team の Team へ移動します。3 件のアイテムは TM の番号になります。" +
        "削除済みの 1 件も一緒に移ります。" +
        "移さないアイテムとの関連を解除します。移すもの同士の関連は残ります。移動先のメンバーにいない人は、移動先のメンバーに加わります。" +
        "このプロジェクトのルールは、移動先の同じ名前のステータス・テンプレートに合わせて移ります。",
      notCarried: [
        "使わないステータス「Pending」(移動先にありません)",
        "使うテンプレート「Bug 報告」(移動先にありません)",
      ],
      replacements: [
        {
          status: "st9",
          name: "Review",
          label: "Review(2 件)",
          options: [
            { value: "st1", label: "Todo" },
            { value: "st2", label: "Doing" },
          ],
          value: "st2",
        },
        {
          status: "st8",
          name: "Pending",
          label: "Pending",
          options: [
            { value: "st1", label: "Todo" },
            { value: "st2", label: "Doing" },
          ],
          value: "st1",
        },
      ],
      ok: "移動",
      busy: false,
    });
  });

  it("accepts only a choice of a row it shows", async () => {
    const { transfer, fake, project } = await setupProject();
    fake.defaultProjectPreview = { ...fake.defaultProjectPreview, replacements: [review] };
    await transfer.load(project);
    await transfer.open();
    transfer.setChoice("st9", "st1");
    transfer.setChoice("st9", "st7");
    transfer.setChoice("st5", "st1");
    expect(get(transfer.dialog)?.replacements.map((r) => r.value)).toEqual(["st1"]);
  });

  it("toasts a refusal or a failure of the preview and shows no dialog", async () => {
    const { transfer, fake, env, project, toasts } = await setupProject();
    await transfer.load(project);
    fake.refuseNext("断りの文");
    await transfer.open();
    fake.failNext();
    await transfer.open();
    expect(get(transfer.dialog)).toBeNull();
    expect(toasts()).toEqual([
      { text: "断りの文", error: true },
      { text: "移動できませんでした", error: true },
    ]);
    expect(env.reloads).toBe(0);
  });

  it("reloads the catalog when the project is gone", async () => {
    const { transfer, fake, env, project } = await setupProject();
    await transfer.load(project);
    fake.missing.add("S1-P1");
    await transfer.open();
    expect(get(transfer.dialog)).toBeNull();
    expect(env.reloads).toBe(1);
  });

  it("reads the preview once when opened again while reading", async () => {
    const { transfer, fake, project } = await setupProject();
    await transfer.load(project);
    const h = fake.holdNext();
    const opening = transfer.open();
    await flush();
    await transfer.open();
    h.release();
    await opening;
    expect(fake.calls.filter((c) => c.startsWith("projectPreview"))).toHaveLength(1);
    expect(get(transfer.dialog)).not.toBeNull();
  });

  it("drops a preview read before cancel", async () => {
    const { transfer, fake, project } = await setupProject();
    await transfer.load(project);
    const h = fake.holdNext();
    const opening = transfer.open();
    await flush();
    transfer.cancel();
    h.release();
    await opening;
    expect(get(transfer.dialog)).toBeNull();
  });
});
