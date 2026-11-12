import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import { text } from "../test-snippet";
import AppFrame from "./AppFrame.svelte";
import PageHead from "./PageHead.svelte";

describe("AppFrame", () => {
  it("renders the sidebar and the main area", () => {
    render(AppFrame, {
      props: {
        drawerOpen: false,
        onclosedrawer: vi.fn(),
        sidebar: text("サイド"),
        children: text("本体"),
      },
    });
    expect(screen.getByRole("complementary")).toHaveTextContent("サイド");
    expect(screen.getByRole("main")).toHaveTextContent("本体");
  });

  it("closes the drawer when the scrim is pressed", async () => {
    const onclosedrawer = vi.fn();
    render(AppFrame, {
      props: { drawerOpen: true, onclosedrawer, sidebar: text("サイド"), children: text("本体") },
    });
    await fireEvent.click(screen.getByRole("button", { name: "メニューを閉じる" }));
    expect(onclosedrawer).toHaveBeenCalledOnce();
  });
});

describe("PageHead", () => {
  it("shows the title and reports the drawer button", async () => {
    const ondrawer = vi.fn();
    render(PageHead, { props: { title: "マイタスク", ondrawer } });
    expect(screen.getByRole("heading", { name: "マイタスク" })).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("button", { name: "メニュー" }));
    expect(ondrawer).toHaveBeenCalledOnce();
  });

  it("shows the note under the title", () => {
    render(PageHead, {
      props: { title: "設定", note: "このマシンの設定と、Store の管理", ondrawer: vi.fn() },
    });
    expect(screen.getByText("このマシンの設定と、Store の管理")).toBeInTheDocument();
  });
});
