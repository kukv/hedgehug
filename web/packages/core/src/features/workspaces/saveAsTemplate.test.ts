import { describe, expect, it } from "vitest";
import { Refused } from "../../model/errors";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createDetail, type DetailView } from "../items/detail";
import { fakeItemsPort, item } from "../items/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { fakeWorkspacesPort } from "./fixtures.test-util";
import { createSaveAsTemplate, saveAsTemplateDialog } from "./saveAsTemplate";

const newToasts = () => createToasts({ set: () => 0, clear: () => {} });

// viewOf は、Store S1 の Task(子は Step)を詳細パネルで開いた状態の DetailView を作る。
async function viewOf(title: string, childTitles: string[]): Promise<DetailView> {
  const catalogPort = fakeCatalogPort();
  const catalog = createCatalog(catalogPort);
  await catalog.load();
  const task = item("S1", "S1-P1", 1, "Task", null, title);
  const steps = childTitles.map((t, i) => item("S1", "S1-P1", i + 2, "Step", task.id, t));
  const detail = createDetail({
    catalog,
    items: fakeItemsPort([task, ...steps]),
    workspaces: fakeWorkspacesPort(),
    catalogPort,
    toasts: newToasts(),
    onChanged: () => {},
  });
  await detail.open("S1", task.id);
  const v = get(detail.view);
  if (!v) throw new Error("detail did not open");
  return v;
}

describe("save as template", () => {
  it("starts from the item's title and hides the structure check without children", async () => {
    expect(saveAsTemplateDialog(await viewOf("ログイン", []))).toEqual({
      name: "ログイン",
      title: "ログイン",
      showStructure: false,
    });
    expect(saveAsTemplateDialog(await viewOf("ログイン", ["設計"])).showStructure).toBe(true);
  });

  it("toasts on success and passes a refusal to the dialog", async () => {
    const port = fakeWorkspacesPort();
    const toasts = newToasts();
    const s = createSaveAsTemplate({ port, toasts });
    const v = await viewOf("x", []);
    const input = { name: "x", title: "x", structure: true };
    expect(await s.save(v, input, () => {})).toBe(true);
    expect(get(toasts.list).at(-1)?.text).toBe("テンプレートを作成しました");
    port.saveItemAsTemplate = async () => {
      throw new Refused("同じ名前のテンプレートがあります");
    };
    let msg = "";
    expect(await s.save(v, input, (m) => (msg = m))).toBe(false);
    expect(msg).toBe("同じ名前のテンプレートがあります");
    expect(get(toasts.list)).toHaveLength(1);
  });

  it("rethrows other errors", async () => {
    const port = fakeWorkspacesPort();
    port.saveItemAsTemplate = async () => {
      throw new Error("boom");
    };
    const s = createSaveAsTemplate({ port, toasts: newToasts() });
    await expect(
      s.save(await viewOf("x", []), { name: "x", title: "x", structure: false }, () => {}),
    ).rejects.toThrow("boom");
  });
});
