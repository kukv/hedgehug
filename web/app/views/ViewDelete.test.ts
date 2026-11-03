import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../core/api/client";
import { cliViewRm } from "../../core/cli";
import { resetToasts, toasts } from "../toast.svelte";
import { runUndo } from "../undo";
import { makeViews, mountWithViews, viewEntry } from "./testkit";
import ViewDelete from "./ViewDelete.svelte";

vi.mock("../undo", () => ({ runUndo: vi.fn(async () => {}) }));

const weekly = viewEntry({ name: "weekly" });

function setup(views = makeViews([weekly])) {
  const onclose = vi.fn();
  const r = mountWithViews(ViewDelete, { entry: weekly, onclose }, { views });
  return { onclose, ...r };
}

describe("ViewDelete", () => {
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/");
  });

  it("確認して消す。足元に hh view rm", async () => {
    const { onclose, views } = setup();
    expect(screen.getByRole("dialog", { name: "ビュー weekly を削除" })).toBeVisible();
    expect(screen.getByText(/shared の views\/weekly.yaml を消します/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      cliViewRm("weekly", "shared"),
    );
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(views.remove).toHaveBeenCalledWith("shared", "weekly");
    expect(toasts.list[0]?.message).toBe("ビュー weekly を削除しました");
  });

  it("トーストの取り消しは、そのビューの Store の直前の操作を戻す", async () => {
    const { onclose } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    await toasts.list[0]?.undo?.();
    expect(runUndo).toHaveBeenCalledWith(expect.anything(), "shared");
  });

  it("開いているのがそのビューの画面なら、ホームへ戻る", async () => {
    history.replaceState(null, "", "/views/shared/weekly");
    const { onclose } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(location.pathname).toBe("/");
  });

  it("ほかの画面にいれば動かさない", async () => {
    history.replaceState(null, "", "/query/table");
    const { onclose } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(location.pathname).toBe("/query/table");
  });

  it("サーバーが断ったら、その言葉を出して開いたままにする", async () => {
    const views = makeViews([weekly]);
    views.remove.mockRejectedValueOnce(
      new ApiError({ status: 404, type: "view.not_found", args: { name: "weekly" } }),
    );
    const { onclose } = setup(views);
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("weekly というビューはありません");
    expect(onclose).not.toHaveBeenCalled();
    expect(toasts.list).toHaveLength(0);
  });
});
