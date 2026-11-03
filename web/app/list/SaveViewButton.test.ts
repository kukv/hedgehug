import { fireEvent, screen } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DEFAULT_KANBAN, DEFAULT_TABLE } from "../../core/views";
import { closeViewDialog, viewDialog } from "../views/dialogs-state.svelte";
import { mountWithViews } from "../views/testkit";
import SaveViewButton from "./SaveViewButton.svelte";
import type { ListScope } from "./scope";

// What useListScope answers is its own test's business: here it is a value the tests set.
const scope = vi.hoisted(() => ({
  ready: true,
  store: "shared" as string | undefined,
  saveBase: "workspace:release",
}));
vi.mock("./scope.svelte", () => ({ useListScope: () => scope }));

const view: ListScope = { kind: "workspace", store: "shared", ulid: "W1" };

beforeEach(() => {
  scope.ready = true;
  scope.store = "shared";
  scope.saveBase = "workspace:release";
});
afterEach(() => closeViewDialog());

describe("SaveViewButton", () => {
  it("固定の部分とフィルタを保存するクエリにして、その画面の Store で始める（D8）", async () => {
    mountWithViews(SaveViewButton, { view, layout: "kanban", filter: "-status:canceled" });
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toEqual({
      kind: "save",
      draft: {
        layout: "kanban",
        query: "workspace:release -status:canceled",
        kanban: DEFAULT_KANBAN,
        table: DEFAULT_TABLE,
      },
      store: "shared",
    });
  });

  it("固定の部分はそのまま使う（Store が2つ以上のときの store: は固定の部分に入っている）", async () => {
    scope.saveBase = "store:shared workspace:release";
    mountWithViews(SaveViewButton, { view, layout: "table", filter: "" });
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toMatchObject({
      draft: { layout: "table", query: "store:shared workspace:release" },
      store: "shared",
    });
  });

  it("プロジェクトの画面の固定の部分は project:<ID>", async () => {
    scope.saveBase = "project:REL-15";
    mountWithViews(SaveViewButton, { view, layout: "tree", filter: "kind:task" });
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toMatchObject({
      draft: { layout: "tree", query: "project:REL-15 kind:task" },
    });
  });

  it("全 Store の画面（固定の部分も Store もない）はフィルタだけで、Store は決めない", async () => {
    scope.saveBase = "";
    scope.store = undefined;
    mountWithViews(
      SaveViewButton,
      { view: { kind: "all" }, layout: "table", filter: "assignee:@me" },
      { stores: ["shared", "me"] },
    );
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toEqual({
      kind: "save",
      draft: expect.objectContaining({ query: "assignee:@me" }),
      store: undefined,
    });
  });

  it("渡された設定をそのまま保存する", async () => {
    mountWithViews(SaveViewButton, {
      view,
      layout: "table",
      filter: "",
      table: { columns: ["id", "title"], groupBy: "status" },
    });
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toMatchObject({
      draft: { table: { columns: ["id", "title"], groupBy: "status" } },
    });
  });

  it("画面の対象がまだ読めないあいだは押せない", () => {
    scope.ready = false;
    mountWithViews(SaveViewButton, { view, layout: "table", filter: "" });
    expect(screen.getByRole("button", { name: "ビューを保存" })).toBeDisabled();
  });
});
