import { describe, expect, it } from "vitest";
import { Refused } from "../../model/errors";
import type { NewProject, Project } from "../../model/project";
import type { ProjectsPort } from "../../port/projects";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort, project } from "../catalog/fixtures.test-util";
import { createProjectDialog } from "./createDialog";

async function setup() {
  const port = fakeCatalogPort();
  const catalog = createCatalog(port);
  await catalog.load();
  const sent: { storeId: string; workspaceId: string; input: NewProject }[] = [];
  const projects: ProjectsPort = {
    get: async () => {
      throw new Error("unused");
    },
    async create(storeId, workspaceId, input) {
      if (input.title.trim() === "") throw new Refused("タイトルを入力してください");
      sent.push({ storeId, workspaceId, input });
      return { ...project(storeId, workspaceId, 3), title: input.title };
    },
  };
  const created: string[] = [];
  const dialog = createProjectDialog({
    catalog,
    catalogPort: port,
    projects,
    onCreated: (s: string, p: Project) => void created.push(`${s}/${p.id}`),
  });
  return { dialog, sent, created };
}

describe("createProjectDialog", () => {
  it("opens on a writable workspace with あなた as the owner and Todo", async () => {
    const { dialog } = await setup();
    await dialog.open("S2-W1"); // 閲覧のみの Store のワークスペースは採らない
    const s = get(dialog.state);
    expect(s?.form).toEqual({
      workspaceId: "S1-W1",
      title: "",
      owner: "taro",
      status: "Todo",
      start: "",
      end: "",
    });
    expect(s?.workspaces).toEqual([{ value: "S1-W1", label: "work / Web" }]);
    expect(s?.owners).toEqual([
      { value: "", label: "なし" },
      { value: "taro", label: "太郎" },
    ]);
  });

  it("shows the refusal in the dialog and keeps the input", async () => {
    const { dialog, sent } = await setup();
    await dialog.open();
    await dialog.update({ title: "  " });
    await dialog.submit();
    expect(get(dialog.state)?.error).toBe("タイトルを入力してください");
    expect(get(dialog.state)?.form.title).toBe("  ");
    expect(sent).toEqual([]);
  });

  it("creates the project, closes, and hands it over", async () => {
    const { dialog, sent, created } = await setup();
    await dialog.open();
    await dialog.update({ title: "決済", status: "InProgress", start: "2026-11-01" });
    await dialog.submit();
    expect(sent).toEqual([
      {
        storeId: "S1",
        workspaceId: "S1-W1",
        input: { title: "決済", owner: "taro", status: "InProgress", start: "2026-11-01", end: "" },
      },
    ]);
    expect(get(dialog.state)).toBeNull();
    expect(created).toEqual(["S1/S1-P3"]);
  });
});
