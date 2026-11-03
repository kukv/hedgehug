import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../core/api/client";
import { cliViewRename } from "../../core/cli";
import { DEFAULT_KANBAN, DEFAULT_TABLE, type ViewDraft } from "../../core/views";
import { resetToasts, toasts } from "../toast.svelte";
import { takeCarriedDraft } from "./dialogs-state.svelte";
import { makeViews, mountWithViews, viewEntry } from "./testkit";
import ViewRename from "./ViewRename.svelte";

const weekly = viewEntry({ name: "weekly" });

const unsaved: ViewDraft = {
  layout: "table",
  query: "assignee:@me",
  kanban: DEFAULT_KANBAN,
  table: DEFAULT_TABLE,
};

function setup(
  views = makeViews([weekly, viewEntry({ name: "Sprint" })]),
  carry: ViewDraft | null = null,
) {
  const onclose = vi.fn();
  const r = mountWithViews(ViewRename, { entry: weekly, carry, onclose }, { views });
  return { onclose, ...r };
}

const typeName = (value: string) =>
  fireEvent.input(screen.getByRole("textbox", { name: "新しい名前" }), { target: { value } });

describe("ViewRename", () => {
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/");
  });

  it("今の名前が入っていて、変えるまで改名できない", () => {
    setup();
    expect(screen.getByRole("dialog", { name: "ビュー weekly を改名" })).toBeVisible();
    expect(screen.getByRole("textbox", { name: "新しい名前" })).toHaveValue("weekly");
    expect(screen.getByRole("button", { name: "改名" })).toBeDisabled();
  });

  it("新しい名前で PATCH して、トーストを出して閉じる。足元に hh view rename", async () => {
    const { onclose, views } = setup();
    await typeName("monthly");
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      cliViewRename("weekly", "monthly", "shared"),
    );
    await fireEvent.click(screen.getByRole("button", { name: "改名" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(views.rename).toHaveBeenCalledWith("shared", "weekly", "monthly");
    expect(toasts.list[0]?.message).toBe("ビュー weekly を monthly に改名しました");
  });

  it("開いているのがそのビューの画面なら、新しい名前の画面に移る（履歴は積まない）", async () => {
    history.replaceState(null, "", "/views/shared/weekly");
    const before = history.length;
    const { onclose } = setup();
    await typeName("monthly");
    await fireEvent.click(screen.getByRole("button", { name: "改名" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(location.pathname).toBe("/views/shared/monthly");
    expect(history.length).toBe(before);
  });

  it("開いているビューの未保存の変更は、新しい名前の画面へ持ち越す", async () => {
    history.replaceState(null, "", "/views/shared/weekly");
    const { onclose } = setup(undefined, unsaved);
    await typeName("monthly");
    await fireEvent.click(screen.getByRole("button", { name: "改名" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(takeCarriedDraft("shared", "monthly")).toEqual(unsaved);
    expect(takeCarriedDraft("shared", "monthly")).toBeNull();
  });

  it("未保存の変更がなければ持ち越さない", async () => {
    history.replaceState(null, "", "/views/shared/weekly");
    const { onclose } = setup();
    await typeName("monthly");
    await fireEvent.click(screen.getByRole("button", { name: "改名" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(takeCarriedDraft("shared", "monthly")).toBeNull();
  });

  it("ほかの画面にいれば動かさない", async () => {
    history.replaceState(null, "", "/views/shared/Sprint");
    const { onclose } = setup();
    await typeName("monthly");
    await fireEvent.click(screen.getByRole("button", { name: "改名" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(location.pathname).toBe("/views/shared/Sprint");
  });

  it("同じ Store の別のビューと重なる名前（大文字小文字の違いも）は断る", async () => {
    setup();
    await typeName("sprint");
    expect(screen.getByText("Sprint というビューが shared にあります")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "改名" })).toBeDisabled();
  });

  it("名前が空なら改名できない", async () => {
    setup();
    await typeName("");
    expect(screen.getByRole("button", { name: "改名" })).toBeDisabled();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toBeDisabled();
  });

  it("サーバーが断ったら、その言葉を出して開いたままにする", async () => {
    const views = makeViews([weekly]);
    views.rename.mockRejectedValueOnce(
      new ApiError({ status: 400, type: "view.bad_name", args: { value: "a/b" } }),
    );
    const { onclose } = setup(views);
    await typeName("a/b");
    await fireEvent.click(screen.getByRole("button", { name: "改名" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("「a/b」はビューの名前に使えません");
    expect(onclose).not.toHaveBeenCalled();
    expect(toasts.list).toHaveLength(0);
  });
});
