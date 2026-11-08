import { cleanup, render } from "@testing-library/svelte";
import { afterEach, describe, expect, it } from "vitest";
import Icon from "./Icon.svelte";
import KindIcon from "./KindIcon.svelte";
import StatusIcon, { shapeOf } from "./StatusIcon.svelte";

afterEach(cleanup);

describe("shapeOf", () => {
  it.each([
    ["backlog", "todo", "dashed"],
    ["todo", "todo", "ring"],
    ["on_hold", "todo", "pause"],
    ["in_progress", "doing", "half"],
    ["waiting", "doing", "clock"],
    ["done", "done", "check"],
    ["canceled", "canceled", "cross"],
  ])("%s (%s) は %s の形", (status, category, shape) => {
    expect(shapeOf(status, category as never)).toBe(shape);
  });

  it("Store が足したステータスはカテゴリの形になる", () => {
    expect(shapeOf("review", "doing")).toBe("half");
    expect(shapeOf("triage", "todo")).toBe("ring");
    expect(shapeOf("shipped", "done")).toBe("check");
    expect(shapeOf("wontfix", "canceled")).toBe("cross");
  });
});

describe("StatusIcon", () => {
  it("形を data-shape に出し、色だけに頼らない", () => {
    const { container } = render(StatusIcon, {
      props: { status: "in_progress", category: "doing" },
    });
    const svg = container.querySelector("svg");
    expect(svg?.getAttribute("data-shape")).toBe("half");
    expect(container.querySelectorAll("circle").length).toBe(1);
    expect(container.querySelector("path")?.getAttribute("d")).toBe("M8 4.5A3.5 3.5 0 0 1 8 11.5Z");
  });

  it("Store が足したステータスの color は使い、形はカテゴリのまま", () => {
    const { container } = render(StatusIcon, {
      props: { status: "review", category: "doing", color: "#0b6b61" },
    });
    const svg = container.querySelector("svg") as SVGElement;
    expect(svg.getAttribute("data-shape")).toBe("half");
    expect(svg.style.getPropertyValue("--c")).toBe("#0b6b61");
  });

  it("壊れた color は無視する", () => {
    const { container } = render(StatusIcon, {
      props: { status: "review", category: "doing", color: "red; background: url(x)" },
    });
    expect((container.querySelector("svg") as SVGElement).style.getPropertyValue("--c")).toBe(
      "var(--accent)",
    );
  });
});

describe("KindIcon", () => {
  it("知っている種類の形を出す。バグだけ danger", () => {
    const bug = render(KindIcon, { props: { kind: "bug" } });
    expect(bug.container.querySelector("svg")?.getAttribute("data-kind")).toBe("bug");
    expect(bug.container.querySelector("svg")?.style.getPropertyValue("--k")).toBe("var(--danger)");
  });

  it("color があればその色で描く。なければ今の色", () => {
    const draw = (props: Record<string, string>) =>
      render(KindIcon, { props: { kind: "task", ...props } }).container.querySelector(
        "svg",
      ) as SVGElement;
    expect(draw({ color: "teal" }).style.getPropertyValue("--k")).toBe("var(--store-teal-dot)");
    cleanup();
    expect(draw({ color: "#3451D1" }).style.getPropertyValue("--k")).toBe("#3451D1");
    cleanup();
    expect(draw({ kind: "bug", color: "blue" }).style.getPropertyValue("--k")).toBe(
      "var(--accent)",
    );
    cleanup();
    expect(draw({}).style.getPropertyValue("--k")).toBe("var(--icon)");
  });

  it("知らない種類は role で container / work の基本形", () => {
    const c = render(KindIcon, { props: { kind: "initiative", role: "container" } });
    expect(c.container.querySelector("svg")?.getAttribute("data-kind")).toBe("container");
    cleanup();
    const w = render(KindIcon, { props: { kind: "chore", role: "work" } });
    expect(w.container.querySelector("svg")?.getAttribute("data-kind")).toBe("work");
  });
});

describe("スキーマの color と icon", () => {
  it("パレットの色で描く", () => {
    const { container } = render(StatusIcon, {
      props: { status: "review", category: "doing", color: "teal" },
    });
    expect((container.querySelector("svg") as SVGElement).style.getPropertyValue("--c")).toBe(
      "var(--store-teal-dot)",
    );
  });

  it("スキーマが名指しした形で描く", () => {
    const { container } = render(KindIcon, {
      props: { kind: "incident", role: "work", icon: "bug" },
    });
    const svg = container.querySelector("svg") as SVGElement;
    expect(svg.dataset.kind).toBe("bug");
    expect(svg.style.getPropertyValue("--k")).toBe("var(--danger)");
  });

  it("知らない icon は無視する", () => {
    const { container } = render(KindIcon, {
      props: { kind: "incident", role: "container", icon: "rocket" },
    });
    expect((container.querySelector("svg") as SVGElement).dataset.kind).toBe("container");
  });
});

describe("Icon", () => {
  it("label がなければ装飾として隠し、あれば img にする", () => {
    const a = render(Icon, { props: { name: "search" } });
    expect(a.container.querySelector("svg")?.getAttribute("aria-hidden")).toBe("true");
    cleanup();
    const b = render(Icon, { props: { name: "search", label: "検索" } });
    expect(b.getByRole("img", { name: "検索" })).toBeTruthy();
  });
});
