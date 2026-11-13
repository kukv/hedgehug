import { render, screen, within } from "@testing-library/svelte";
import { describe, expect, it } from "vitest";
import type { MarkdownBlock } from "../types";
import Markdown from "./Markdown.svelte";

const text = (s: string) => ({ kind: "text" as const, text: s });

const blocks: MarkdownBlock[] = [
  { kind: "heading", level: 1, inlines: [text("概要")] },
  {
    kind: "paragraph",
    inlines: [
      { kind: "bold", text: "太字" },
      text(" と "),
      { kind: "code", text: "go test" },
      text(" と "),
      { kind: "link", url: "https://example.com/a" },
      text(" と "),
      { kind: "mention", login: "taro" },
    ],
  },
  {
    kind: "list",
    ordered: false,
    items: [
      { check: "unchecked", inlines: [text("未")] },
      { check: "checked", inlines: [text("済")] },
      { check: "none", inlines: [text("印なし")] },
    ],
  },
  { kind: "list", ordered: true, items: [{ check: "none", inlines: [text("手順")] }] },
  { kind: "code", text: "<script>alert(1)</script>" },
];

describe("Markdown", () => {
  it("renders every kind of block and inline", () => {
    const { container } = render(Markdown, { props: { blocks } });
    // 見出しは、区画の見出し(h3)の下になるよう h4 から。
    expect(screen.getByRole("heading", { level: 4, name: "概要" })).toBeInTheDocument();
    const para = container.querySelector("p");
    expect(para?.textContent).toBe("太字 と go test と https://example.com/a と @taro");
    expect(container.querySelector("strong")).toHaveTextContent("太字");
    expect(container.querySelector("p code")).toHaveTextContent("go test");
    expect(screen.getByText("@taro")).toHaveClass("mention");
    expect(screen.getByText("@taro")).toHaveAttribute("data-mention", "taro");
    const lists = screen.getAllByRole("list");
    expect(lists).toHaveLength(2);
    expect(within(lists[0] as HTMLElement).getAllByRole("listitem")).toHaveLength(3);
    expect(container.querySelector("ol")).toHaveTextContent("手順");
    expect(screen.getByRole("img", { name: "未チェック" })).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "チェック済み" })).toBeInTheDocument();
  });

  it("puts the lower-case login on a mention as data-mention", () => {
    const { container } = render(Markdown, {
      props: { blocks: [{ kind: "paragraph", inlines: [{ kind: "mention", login: "TARO" }] }] },
    });
    const mention = container.querySelector(".mention");
    expect(mention).toHaveTextContent("@TARO");
    expect(mention).toHaveAttribute("data-mention", "taro");
  });

  it("opens links in a new tab without leaking the opener", () => {
    render(Markdown, { props: { blocks } });
    const link = screen.getByRole("link", { name: "https://example.com/a" });
    expect(link).toHaveAttribute("href", "https://example.com/a");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("shows markup as text and never makes an element from it (XSS)", () => {
    const { container } = render(Markdown, { props: { blocks } });
    expect(container.querySelector("script")).toBeNull();
    expect(screen.getByText("<script>alert(1)</script>")).toBeInTheDocument();
  });
});
