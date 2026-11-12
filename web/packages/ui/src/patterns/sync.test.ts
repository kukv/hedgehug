import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import type { SyncCardView } from "../types";
import OfflineBand from "./OfflineBand.svelte";
import SyncCard from "./SyncCard.svelte";

const card: SyncCardView = {
  summary: "1 件の Store で同期エラー",
  error: true,
  last: "最終同期 2 時間前 · Store 2 件",
  stale: "1 件の Store が 24 時間以上同期されていません",
  rows: [
    {
      storeId: "S1",
      name: "team-a",
      summary: "同期エラー: 認証に失敗しました",
      time: "2 時間前",
      stale: false,
    },
    { storeId: "S2", name: "solo", summary: "未送信 3", time: "1 日前", stale: true },
  ],
};

describe("SyncCard", () => {
  it("shows the lines and the hint of 「すべて同期」", async () => {
    const onsyncall = vi.fn();
    render(SyncCard, { props: { card, hint: "Shift+Ctrl+S", onsyncall, onsyncstore: vi.fn() } });
    expect(screen.getByRole("button", { name: "1 件の Store で同期エラー" })).toBeInTheDocument();
    expect(screen.getByText("最終同期 2 時間前 · Store 2 件")).toBeInTheDocument();
    expect(screen.getByText("1 件の Store が 24 時間以上同期されていません")).toBeInTheDocument();
    const all = screen.getByRole("button", { name: "すべて同期" });
    expect(all).toHaveAttribute("title", "Shift+Ctrl+S");
    await fireEvent.click(all);
    expect(onsyncall).toHaveBeenCalledOnce();
  });

  it("opens 「Store ごとの同期状態」 and syncs the chosen store", async () => {
    const onsyncstore = vi.fn();
    render(SyncCard, { props: { card, hint: "", onsyncall: vi.fn(), onsyncstore } });
    await fireEvent.click(screen.getByRole("button", { name: "1 件の Store で同期エラー" }));
    const menu = screen.getByRole("menu", { name: "Store ごとの同期状態" });
    expect(menu).toHaveTextContent("team-a");
    expect(menu).toHaveTextContent("同期エラー: 認証に失敗しました · 2 時間前");
    await fireEvent.click(screen.getByRole("menuitem", { name: /solo/ }));
    expect(onsyncstore).toHaveBeenCalledWith("S2");
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("closes the menu with Escape", async () => {
    render(SyncCard, { props: { card, hint: "", onsyncall: vi.fn(), onsyncstore: vi.fn() } });
    await fireEvent.click(screen.getByRole("button", { name: "1 件の Store で同期エラー" }));
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });
});

describe("OfflineBand", () => {
  it("shows the text as a status", () => {
    render(OfflineBand, { props: { text: "オフラインです。" } });
    expect(screen.getByRole("status")).toHaveTextContent("オフラインです。");
  });
});
