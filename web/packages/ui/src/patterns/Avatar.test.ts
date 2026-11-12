import { render, screen } from "@testing-library/svelte";
import { describe, expect, it } from "vitest";
import Avatar from "./Avatar.svelte";

describe("Avatar", () => {
  it("shows the first letter and the name as the title", () => {
    render(Avatar, { props: { name: "太郎", title: "太郎" } });
    expect(screen.getByTitle("太郎")).toHaveTextContent("太");
  });

  it("is empty for an unknown user", () => {
    render(Avatar, { props: { title: "不明なユーザー" } });
    // Svelte は空の文字ノードを残すので、toBeEmptyDOMElement ではなく textContent で見る。
    expect(screen.getByTitle("不明なユーザー").textContent).toBe("");
  });
});
