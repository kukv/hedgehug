import { fireEvent, screen } from "@testing-library/svelte";
import { afterEach, beforeAll, describe, expect, it } from "vitest";
import { closeWorkspaceDialog } from "../workspace/dialogs-state.svelte";
import TaskDetail from "./TaskDetail.svelte";
import { makeCore, mount, showView, stubLayout, taskView } from "./testkit";

const U = "01JBZ8K3QW7M2X9V4T6R5N8P0A";
const SENTENCE = "この Store は閲覧のみです（remote に書き込めません）";
const show = showView({
  ulid: U,
  id: "REL-6",
  title: "Web の API クライアントを v2 に更新",
  body: "- [ ] a\n",
  workspace: { ulid: "W1", name: "release", id_prefix: "REL" },
});

function setup(readOnly?: string, over: Partial<typeof show> = {}) {
  const k = makeCore({ show: { ...show, ...over }, tasks: [taskView()], readOnly });
  return { k, ...mount(TaskDetail, { ulid: U }, k) };
}

describe("TaskDetail of a read-only Store", () => {
  beforeAll(stubLayout);
  afterEach(() => closeWorkspaceDialog());

  it("shows the band with the reason", () => {
    setup("the remote cannot be written");
    expect(screen.getByRole("status", { name: "閲覧のみ" })).toHaveTextContent(SENTENCE);
  });

  it("only reads the title, the checklist and the comments", () => {
    setup("the remote cannot be written");
    expect(screen.getByRole("textbox", { name: "タイトル" })).toHaveAttribute("readonly");
    expect(screen.getByRole("checkbox", { name: "a" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "コメント" })).toBeNull();
  });

  it("turns the watch, delete and move off, and says why", async () => {
    setup("the remote cannot be written");
    const watch = screen.getByRole("button", { name: "ウォッチ" });
    expect(watch).toBeDisabled();
    expect(watch).toHaveAttribute("title", SENTENCE);
    const del = screen.getByRole("button", { name: "削除" });
    expect(del).toBeDisabled();
    expect(del).toHaveAttribute("title", SENTENCE);
    await fireEvent.click(screen.getByRole("button", { name: "その他の操作" }));
    const item = await screen.findByRole("menuitem", { name: /別のワークスペースへ移動/ });
    expect(item).toBeDisabled();
    expect(item).toHaveTextContent(SENTENCE);
  });

  it("turns the restore of a deleted task off", () => {
    setup("the remote cannot be written", { deleted_at: "2026-11-03T10:00:00Z" });
    expect(screen.getByRole("button", { name: "復元" })).toBeDisabled();
  });

  it("stays as it was for a Store that can be written", () => {
    setup();
    expect(screen.queryByRole("status", { name: "閲覧のみ" })).toBeNull();
    expect(screen.getByRole("textbox", { name: "タイトル" })).not.toHaveAttribute("readonly");
    expect(screen.getByRole("button", { name: "ウォッチ" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "削除" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "コメント" })).toBeInTheDocument();
  });
});
