import { fireEvent, screen } from "@testing-library/svelte";
import { describe, expect, it } from "vitest";
import PropsPanel from "./PropsPanel.svelte";
import { makeCore, mount, showView } from "./testkit";

const show = showView({
  ulid: "U6",
  id: "REL-6",
  store: "shared",
  kind: "task",
  status: "in_progress",
  assignees: ["sato"],
  labels: ["frontend"],
  watchers: ["koki", "sato"],
  props: { priority: "mid", due: "2026-11-07", points: 5, team: "web" },
});

describe("PropsPanel", () => {
  it("shows the built-in fields and the properties of the schema", () => {
    mount(PropsPanel, { show }, makeCore({ show }));
    expect(screen.getByRole("button", { name: "ステータス" })).toHaveTextContent("進行中 · doing");
    expect(screen.getByRole("button", { name: "種類" })).toHaveTextContent("タスク");
    expect(screen.getByRole("button", { name: "priority" })).toHaveTextContent("mid");
    expect(screen.getByLabelText("due")).toHaveValue("2026-11-07");
    expect(screen.getByRole("spinbutton", { name: "points" })).toHaveValue(5);
    expect(screen.getByText("shared")).toBeInTheDocument();
  });

  it("shows a property the schema does not define as read-only, in italics", () => {
    mount(PropsPanel, { show }, makeCore({ show }));
    expect(screen.getByText("team（未定義）")).toHaveClass("undef");
    expect(screen.getByText("web")).toBeInTheDocument();
  });

  it("edits through the SaveController and flushes for discrete choices", async () => {
    const k = makeCore({ show });
    mount(PropsPanel, { show }, k);
    await fireEvent.click(screen.getByRole("button", { name: "ステータス" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "完了 · done" }));
    expect(k.save.edit).toHaveBeenCalledWith("U6", "shared", "status", "done");
    expect(k.save.flush).toHaveBeenCalledWith("U6");
    await fireEvent.change(screen.getByLabelText("due"), { target: { value: "2026-11-09" } });
    expect(k.save.edit).toHaveBeenCalledWith("U6", "shared", "props.due", "2026-11-09");
  });

  it("edits a number while typing and commits on blur", async () => {
    const k = makeCore({ show });
    mount(PropsPanel, { show }, k);
    const points = screen.getByRole("spinbutton", { name: "points" });
    await fireEvent.input(points, { target: { value: "8" } });
    expect(k.save.edit).toHaveBeenCalledWith("U6", "shared", "props.points", 8);
    expect(k.save.flush).not.toHaveBeenCalled();
    await fireEvent.blur(points);
    expect(k.save.flush).toHaveBeenCalledWith("U6");
  });

  it("adds an assignee and a label", async () => {
    const k = makeCore({ show });
    mount(PropsPanel, { show }, k);
    const assignee = screen.getByRole("textbox", { name: "担当者" });
    await fireEvent.input(assignee, { target: { value: "koki" } });
    await fireEvent.keyDown(assignee, { key: "Enter" });
    expect(k.save.edit).toHaveBeenCalledWith("U6", "shared", "assignees", ["sato", "koki"]);
    const label = screen.getByRole("textbox", { name: "ラベル" });
    await fireEvent.input(label, { target: { value: "v2" } });
    await fireEvent.keyDown(label, { key: "Enter" });
    expect(k.save.edit).toHaveBeenCalledWith("U6", "shared", "labels", ["frontend", "v2"]);
  });

  it("does not offer a kind the parent cannot hold", async () => {
    const child = showView({ ...show, parent: "UP" });
    const k = makeCore({ show: child });
    k.cache.task.mockImplementation((u: string) =>
      u === "UP" ? ({ ulid: "UP", kind: "task" } as never) : undefined,
    );
    mount(PropsPanel, { show: child }, k);
    await fireEvent.click(screen.getByRole("button", { name: "種類" }));
    expect(screen.getByRole("menuitem", { name: /^プロジェクト/ })).toBeDisabled();
  });

  it("takes the kind of the parent the screen read when the cache lacks the parent", async () => {
    const child = showView({ ...show, parent: "UP" });
    mount(PropsPanel, { show: child, parentKind: "task" }, makeCore({ show: child }));
    await fireEvent.click(screen.getByRole("button", { name: "種類" }));
    expect(screen.getByRole("menuitem", { name: /^プロジェクト/ })).toBeDisabled();
  });

  it("rules no kind out while the kind of the parent is not known", async () => {
    const child = showView({ ...show, parent: "UP" });
    mount(PropsPanel, { show: child }, makeCore({ show: child }));
    await fireEvent.click(screen.getByRole("button", { name: "種類" }));
    expect(screen.getByRole("menuitem", { name: /^プロジェクト/ })).not.toBeDisabled();
  });

  it("does not edit when read-only", () => {
    mount(PropsPanel, { show, readonly: true }, makeCore({ show }));
    expect(screen.getByRole("button", { name: "ステータス" })).toBeDisabled();
    expect(screen.getByLabelText("due")).toHaveAttribute("readonly");
  });

  it("lists the watchers", () => {
    mount(PropsPanel, { show }, makeCore({ show }));
    expect(screen.getByRole("list", { name: "ウォッチャー" })).toHaveTextContent("koki");
  });
});
