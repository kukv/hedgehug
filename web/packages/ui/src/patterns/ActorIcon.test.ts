import { render, screen } from "@testing-library/svelte";
import { describe, expect, it } from "vitest";
import ActorIcon from "./ActorIcon.svelte";

describe("ActorIcon", () => {
  it("shows the avatar for a user", () => {
    render(ActorIcon, { props: { kind: "user", name: "太郎", title: "太郎" } });
    expect(screen.getByRole("img", { name: "太郎" })).toHaveTextContent("太");
  });

  it("is an empty circle for a user without a name", () => {
    render(ActorIcon, { props: { kind: "user", name: "", title: "不明なユーザー" } });
    expect(screen.getByRole("img", { name: "不明なユーザー" }).textContent).toBe("");
  });

  it("shows A for automation", () => {
    render(ActorIcon, { props: { kind: "automation", name: "", title: "自動化" } });
    expect(screen.getByRole("img", { name: "自動化" })).toHaveTextContent("A");
  });

  it("shows hh for none", () => {
    render(ActorIcon, { props: { kind: "none", name: "", title: "hedgehug" } });
    expect(screen.getByRole("img", { name: "hedgehug" })).toHaveTextContent("hh");
  });

  it("draws an svg mark for github and sync", () => {
    render(ActorIcon, { props: { kind: "github", name: "", title: "GitHub" } });
    render(ActorIcon, { props: { kind: "sync", name: "", title: "同期" } });
    for (const [name, glyph] of [
      ["GitHub", "github"],
      ["同期", "sync"],
    ]) {
      const icon = screen.getByRole("img", { name });
      expect(icon.querySelector("svg")?.getAttribute("data-glyph")).toBe(glyph);
      expect(icon.textContent?.trim()).toBe("");
    }
  });
});
