import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import ConfirmDialog from "./ConfirmDialog.svelte";

describe("ConfirmDialog", () => {
  it("shows a paragraph per line and answers with the buttons and Escape", async () => {
    const onok = vi.fn();
    const oncancel = vi.fn();
    render(ConfirmDialog, {
      props: {
        title: "アカウントを削除",
        lines: [
          "「work」を削除します。",
          "チーム の同期に使っているため、それらは同期できなくなります。",
        ],
        ok: "削除",
        danger: true,
        onok,
        oncancel,
      },
    });
    const dialog = screen.getByRole("dialog", { name: "アカウントを削除" });
    expect(dialog.querySelectorAll("p")).toHaveLength(2);
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    expect(onok).toHaveBeenCalledOnce();
    await fireEvent.click(screen.getByRole("button", { name: "キャンセル" }));
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(oncancel).toHaveBeenCalledTimes(2);
  });
});
