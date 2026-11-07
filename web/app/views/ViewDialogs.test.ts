import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { afterEach, describe, expect, it } from "vitest";
import { DEFAULT_GANTT, DEFAULT_KANBAN, DEFAULT_TABLE, type ViewDraft } from "../../core/views";
import {
  closeViewDialog,
  openViewDelete,
  openViewRename,
  openViewSave,
  viewDialog,
} from "./dialogs-state.svelte";
import { mountWithViews, viewEntry } from "./testkit";
import ViewDialogs from "./ViewDialogs.svelte";

const draft: ViewDraft = {
  layout: "kanban",
  query: "assignee:@me",
  kanban: DEFAULT_KANBAN,
  gantt: DEFAULT_GANTT,
  table: DEFAULT_TABLE,
};

afterEach(() => closeViewDialog());

describe("ViewDialogs", () => {
  it("何も頼まれていなければ何も出さない", () => {
    mountWithViews(ViewDialogs, {});
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("保存・改名・削除を頼まれた順に出し、閉じるとなくなる", async () => {
    mountWithViews(ViewDialogs, {});
    const entry = viewEntry({ name: "weekly" });

    openViewSave({ draft, store: "shared" });
    expect(await screen.findByRole("dialog", { name: "ビューを保存" })).toBeVisible();
    closeViewDialog();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

    openViewRename(entry);
    expect(await screen.findByRole("dialog", { name: "ビュー weekly の名前を変更" })).toBeVisible();

    openViewDelete(entry);
    expect(await screen.findByRole("dialog", { name: "ビュー weekly を削除" })).toBeVisible();
    await fireEvent.click(screen.getByRole("button", { name: "キャンセル" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(viewDialog.current).toBeNull();
  });

  it("保存のダイアログは渡された Store で始まる", async () => {
    mountWithViews(ViewDialogs, {}, { stores: ["shared", "me"] });
    openViewSave({ draft, store: "me" });
    expect(await screen.findByRole("radio", { name: /me/ })).toBeChecked();
  });
});
