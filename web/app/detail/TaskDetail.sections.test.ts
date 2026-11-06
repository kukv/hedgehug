import { screen } from "@testing-library/svelte";
import { beforeAll, describe, expect, it } from "vitest";
import TaskDetail from "./TaskDetail.svelte";
import { makeCore, mount, ok, showView, stubLayout } from "./testkit";

const U = "01JBZ8K3QW7M2X9V4T6R5N8P0A";
const base = showView({ ulid: U, id: "REL-6", store: "shared", title: "t", parent: null });

function setup(over: Partial<typeof base> = {}) {
  const k = makeCore({ show: { ...base, ...over } });
  k.api.getGithub.mockImplementation(() => ok([]));
  k.api.getTaskHistory.mockImplementation(() => ok([]));
  k.api.listPeople.mockImplementation(() => ok([]));
  return mount(TaskDetail, { ulid: U }, k);
}

describe("TaskDetail sections", () => {
  beforeAll(stubLayout);

  it("has relations, comments, history and the delete button for a live task", async () => {
    setup();
    expect(await screen.findByRole("heading", { name: "関連" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /コメント/ })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "履歴" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "削除" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "復元" })).toBeNull();
  });

  it("has restore instead of delete for a deleted task", async () => {
    setup({ deleted_at: "2026-11-03T10:00:00Z" });
    expect(await screen.findByRole("button", { name: "復元" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "完全に削除" })).toBeNull();
    expect(screen.queryByRole("button", { name: "削除" })).toBeNull();
  });
});
