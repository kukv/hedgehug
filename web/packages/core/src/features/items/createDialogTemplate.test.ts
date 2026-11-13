import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { setup } from "./createDialogTemplate.test-util";

describe("create dialog templates", () => {
  it("applies the kind's default when it opens", async () => {
    const { dialog } = await setup();
    const s = get(dialog.state);
    expect(s?.form).toMatchObject({ kind: "Task", templateId: "T1", title: "調査: " });
    expect(s?.form.values.priority).toBe("pr1");
    expect(s?.templates.map((o) => o.label)).toEqual(["なし", "調査", "作業"]);
  });

  it("switching the kind drops the template and applies the new default; the title stays", async () => {
    const { dialog } = await setup();
    await dialog.update({ kind: "Bug" });
    expect(get(dialog.state)?.form).toMatchObject({ templateId: "B1", title: "調査: " });
    await dialog.update({ kind: "Story" });
    expect(get(dialog.state)?.form.templateId).toBe("");
    expect(get(dialog.state)?.templates).toEqual([]);
  });

  it("selecting a template overwrites its values and shows what is made with it", async () => {
    const { dialog } = await setup();
    const values = get(dialog.state)?.form.values ?? {};
    await dialog.update({ values: { ...values, assignee: "taro" } });
    await dialog.update({ templateId: "T2" });
    const s = get(dialog.state);
    expect(s?.form.templateId).toBe("T2");
    expect(s?.structure).toEqual({
      heading: "一緒に作るもの: Step 1 件",
      rows: [{ depth: 0, kind: "Step", title: "準備", assignee: "太郎" }],
    });
  });

  it("changing the workspace applies the new workspace's default after resetting values", async () => {
    const { dialog } = await setup();
    await dialog.update({ workspaceId: "S2-W1" });
    const s = get(dialog.state);
    expect(s?.form.templateId).toBe("X1");
    expect(s?.form.values.assignee).toBe("jiro");
    expect(s?.form.values.priority).toBe("np1");
  });

  it("sends the template, toasts the others and keeps it when continuing", async () => {
    const { dialog, items, shown } = await setup();
    await dialog.update({ templateId: "T2", title: "作業する", keepOpen: true });
    await dialog.submit();
    expect(items.createdTemplates.at(-1)).toBe("T2");
    expect(shown().at(-1)).toMatch(/^HH-\d+ ほか 1 件を作成しました$/);
    // 続けて作成: タイトルだけ空にし、テンプレートと「一緒に作るもの」は残す。書き出しは入れ直さない。
    const s = get(dialog.state);
    expect(s?.form).toMatchObject({ templateId: "T2", title: "" });
    expect(s?.structure?.rows).toHaveLength(1);
  });

  it("sends no template when none is chosen", async () => {
    const { dialog, items, shown } = await setup();
    await dialog.update({ templateId: "", title: "なしで作る" });
    await dialog.submit();
    expect(items.createdTemplates.at(-1)).toBeUndefined();
    expect(shown().at(-1)).toMatch(/^HH-\d+ を作成しました$/);
  });

  it("keeps the kind and the template together when the kind changes during a reload", async () => {
    const { dialog } = await setup();
    const reloading = dialog.update({ projectId: "S1-P2" });
    await dialog.update({ kind: "Bug" });
    await reloading;
    const s = get(dialog.state);
    expect(s?.form).toMatchObject({ kind: "Bug", templateId: "B1" });
    const usable = (s?.templates ?? []).map((o) => o.value);
    expect(s?.form.templateId === "" || usable.includes(s?.form.templateId ?? "")).toBe(true);
    expect(
      s?.settings.templates.find((t) => t.id === s?.form.templateId)?.kind ?? s?.form.kind,
    ).toBe(s?.form.kind);
  });

  it("does not put the template's title back after continuing or a project change", async () => {
    const { dialog } = await setup();
    await dialog.update({ title: "調べる", keepOpen: true });
    await dialog.submit();
    expect(get(dialog.state)?.form).toMatchObject({ templateId: "T1", title: "" });
    await dialog.update({ projectId: "S1-P2" });
    expect(get(dialog.state)?.form).toMatchObject({ templateId: "T1", title: "" });
  });
});

describe("create dialog description", () => {
  const description = (dialog: Awaited<ReturnType<typeof setup>>["dialog"]) =>
    get(dialog.state)?.form.description;

  it("fills the default template's description when it opens, and sends it", async () => {
    const { dialog, items } = await setup();
    expect(description(dialog)).toBe("## 調査の手順");
    await dialog.update({ title: "調べる" });
    await dialog.submit();
    expect(items.createdDescriptions.at(-1)).toBe("## 調査の手順");
  });

  it("replaces the template's own description when the kind changes, and keeps a typed one", async () => {
    const { dialog } = await setup();
    await dialog.update({ kind: "Bug" });
    expect(description(dialog)).toBe("## 不具合の概要");
    await dialog.update({ description: "## 不具合の概要!" });
    await dialog.update({ kind: "Task" });
    expect(description(dialog)).toBe("## 不具合の概要!");
  });

  it("replaces the template's own description when the workspace changes, and keeps a typed one", async () => {
    const { dialog } = await setup();
    await dialog.update({ workspaceId: "S2-W1" });
    expect(description(dialog)).toBe("## 別の手順");
    await dialog.update({ workspaceId: "S1-W1" });
    await dialog.update({ description: "メモ" });
    await dialog.update({ workspaceId: "S2-W1" });
    expect(description(dialog)).toBe("メモ");
  });

  it("select replaces a typed description; なし clears only a template's own", async () => {
    const { dialog } = await setup();
    await dialog.update({ description: "メモ" });
    await dialog.update({ templateId: "T2" });
    expect(description(dialog)).toBe("## 作業の手順");
    await dialog.update({ templateId: "" });
    expect(description(dialog)).toBe("");
    await dialog.update({ templateId: "T1" });
    await dialog.update({ description: "メモ" });
    await dialog.update({ templateId: "" });
    expect(description(dialog)).toBe("メモ");
  });

  it("continuing clears the title and the description, and keeps the template", async () => {
    const { dialog } = await setup();
    await dialog.update({ title: "続けて", keepOpen: true });
    await dialog.submit();
    expect(get(dialog.state)?.form).toMatchObject({ title: "", description: "", templateId: "T1" });
  });

  it("keeps a description typed while the dialog reloads", async () => {
    const { dialog } = await setup();
    const reloading = dialog.update({ projectId: "S1-P2" });
    await dialog.update({ description: "読み直し中に打った" });
    await reloading;
    expect(description(dialog)).toBe("読み直し中に打った");
  });
});
