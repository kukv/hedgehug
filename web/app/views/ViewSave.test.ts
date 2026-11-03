import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { cliViewSave } from "../../core/cli";
import { DEFAULT_KANBAN, saveBodyOf, type ViewDraft } from "../../core/views";
import { resetToasts, toasts } from "../toast.svelte";
import { makeViews, mountWithViews, viewEntry } from "./testkit";
import ViewSave from "./ViewSave.svelte";

const table: ViewDraft = {
  layout: "table",
  query: "workspace:release -status:canceled",
  kanban: DEFAULT_KANBAN,
  table: { columns: ["id", "title", "status"], groupBy: "parent" },
};

function setup(
  over: {
    draft?: ViewDraft;
    store?: string;
    stores?: string[];
    readOnly?: string[];
    views?: ReturnType<typeof makeViews>;
  } = {},
) {
  const onclose = vi.fn();
  const r = mountWithViews(
    ViewSave,
    { draft: over.draft ?? table, store: over.store, onclose },
    { stores: over.stores, readOnly: over.readOnly, views: over.views },
  );
  return { onclose, ...r };
}

const typeName = (value: string) =>
  fireEvent.input(screen.getByRole("textbox", { name: "名前" }), { target: { value } });

describe("ViewSave", () => {
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/query/table");
  });

  it("名前を入れて保存すると、今の設定をまるごと PUT して、保存したビューを開く", async () => {
    const { onclose, views } = setup();
    expect(screen.getByRole("dialog", { name: "ビューを保存" })).toBeVisible();
    await typeName("weekly");
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(views.save).toHaveBeenCalledWith("shared", "weekly", saveBodyOf(table));
    expect(location.pathname).toBe("/views/shared/weekly");
    expect(toasts.list[0]?.message).toBe("ビュー weekly を shared に保存しました");
  });

  it("足元に hh view save を出す。名前が空のあいだは <名前> でコピーできない", async () => {
    setup();
    const cli = () => screen.getByRole("button", { name: /コマンドをコピー/ });
    expect(cli()).toBeDisabled();
    expect(cli()).toHaveTextContent("<名前>");
    await typeName("weekly");
    expect(cli()).toBeEnabled();
    expect(cli()).toHaveTextContent(
      cliViewSave({ name: "weekly", store: "shared", body: saveBodyOf(table) }),
    );
  });

  it("設定の要約とクエリを出す", () => {
    setup();
    expect(screen.getByText("テーブル · 列 3 · グループ: 親")).toBeInTheDocument();
    expect(screen.getByText("workspace:release -status:canceled")).toBeInTheDocument();
  });

  it("名前が空なら保存できない。最初は叱らない", () => {
    const { views } = setup();
    expect(screen.getByRole("button", { name: "保存" })).toBeDisabled();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(views.save).not.toHaveBeenCalled();
  });

  it("Store が1つなら保存先の欄を出さず、2つ以上なら選ばせる（既定は default の Store）", async () => {
    const one = setup();
    expect(screen.queryByRole("radiogroup", { name: "保存先" })).toBeNull();
    one.unmount();

    setup({ stores: ["shared", "me"] });
    const group = screen.getByRole("radiogroup", { name: "保存先" });
    expect(group).toBeVisible();
    expect(screen.getByRole("radio", { name: /shared/ })).toBeChecked();
    await fireEvent.click(screen.getByRole("radio", { name: /me/ }));
    await typeName("mine");
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(location.pathname).toBe("/views/me/mine"));
  });

  it("画面の Store が渡されていれば、それが最初の選択", () => {
    setup({ stores: ["shared", "me"], store: "me" });
    expect(screen.getByRole("radio", { name: /me/ })).toBeChecked();
  });

  it("閲覧のみの Store は選べない", () => {
    setup({ stores: ["shared", "ro"], readOnly: ["ro"] });
    expect(screen.getByRole("radio", { name: /ro/ })).toBeDisabled();
    expect(screen.getByRole("radio", { name: /shared/ })).toBeEnabled();
  });

  it("同じ Store に同じ名前（大文字小文字の違いも）があれば保存できず、理由を出す", async () => {
    const views = makeViews([viewEntry({ name: "Weekly" })]);
    const { onclose } = setup({ views });
    await typeName("weekly");
    expect(screen.getByText(/Weekly というビューが shared にあります/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "保存" })).toBeDisabled();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toBeDisabled();
    expect(onclose).not.toHaveBeenCalled();
  });

  it("別の Store に同じ名前があってもよい（細部 53）", async () => {
    const views = makeViews([viewEntry({ name: "weekly" }, "other")]);
    setup({ views });
    await typeName("weekly");
    expect(screen.getByRole("button", { name: "保存" })).toBeEnabled();
  });

  it("ツリーで sort: か group: があれば、保存の前に理由を出して保存を無効にする（細部 54）", async () => {
    setup({ draft: { ...table, layout: "tree", query: "kind:task sort:updated" } });
    await typeName("weekly");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "ツリーのビューは rank の順に並ぶため、group: と sort: は使えません",
    );
    expect(screen.getByRole("button", { name: "保存" })).toBeDisabled();
  });

  it("カンバンやテーブルなら sort: があっても保存できる", async () => {
    setup({ draft: { ...table, query: "sort:updated" } });
    await typeName("weekly");
    expect(screen.getByRole("button", { name: "保存" })).toBeEnabled();
  });

  it("サーバーが断ったら、その言葉を出してダイアログを開いたままにする", async () => {
    const views = makeViews();
    const { ApiError } = await import("../../core/api/client");
    views.save.mockRejectedValueOnce(
      new ApiError({ status: 422, type: "view.bad_name", args: { value: "a/b" } }),
    );
    const { onclose } = setup({ views });
    await typeName("a/b");
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("「a/b」はビューの名前に使えません");
    expect(onclose).not.toHaveBeenCalled();
    expect(location.pathname).toBe("/query/table");
    expect(toasts.list).toHaveLength(0);
  });

  it("名前の欄で Enter を押すと保存する。変換中の Enter は保存しない", async () => {
    const { onclose, views } = setup();
    await typeName("weekly");
    const box = screen.getByRole("textbox", { name: "名前" });
    await fireEvent.keyDown(box, { key: "Enter", isComposing: true });
    expect(views.save).not.toHaveBeenCalled();
    await fireEvent.keyDown(box, { key: "Enter" });
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(views.save).toHaveBeenCalledTimes(1);
  });

  it("書き込みのあいだは閉じない", async () => {
    const views = makeViews();
    let release: (() => void) | undefined;
    views.save.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          release = () => resolve(viewEntry({ name: "weekly" }).view);
        }),
    );
    const { onclose } = setup({ views });
    await typeName("weekly");
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await fireEvent.click(screen.getByRole("button", { name: "キャンセル" }));
    expect(onclose).not.toHaveBeenCalled();
    release?.();
    await waitFor(() => expect(onclose).toHaveBeenCalled());
  });
});
