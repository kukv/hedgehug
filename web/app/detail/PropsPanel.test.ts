import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { describe, expect, it } from "vitest";
import CreateHost from "../create/CreateHost.svelte";
import PropsPanel from "./PropsPanel.svelte";
import { makeCore, mount, schemaView, showView } from "./testkit";

const show = showView({
  ulid: "U6",
  id: "REL-6",
  store: "shared",
  kind: "task",
  status: "in_progress",
  assignees: ["sato"],
  labels: ["bug"],
  watchers: ["koki", "sato"],
  priority: "mid",
  due: "2026-11-07",
  estimate: "1h30m",
  points: 5,
  props: { tier: "gold", kickoff: "2026-11-01", team: "web" },
});

describe("PropsPanel", () => {
  it("shows the built-in fields and the properties of the schema", () => {
    mount(PropsPanel, { show }, makeCore({ show }));
    expect(screen.getByRole("button", { name: "ステータス" })).toHaveTextContent("進行中 · doing");
    expect(screen.getByRole("button", { name: "種類" })).toHaveTextContent("Task");
    expect(screen.getByRole("button", { name: "優先度" })).toHaveTextContent("mid");
    expect(screen.getByLabelText("期限")).toHaveValue("2026-11-07");
    expect(screen.getByLabelText("開始日")).toHaveValue("");
    expect(screen.getByRole("textbox", { name: "見積もり" })).toHaveValue("1h30m");
    expect(screen.getByRole("spinbutton", { name: "ポイント" })).toHaveValue(5);
    expect(screen.getByRole("button", { name: "bug を外す" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "tier" })).toHaveTextContent("gold");
    expect(screen.getByLabelText("kickoff")).toHaveValue("2026-11-01");
    expect(screen.getByText("shared")).toBeInTheDocument();
  });

  it("puts the built-in properties below the status in a fixed order, before the properties of the schema", () => {
    const { container } = mount(PropsPanel, { show }, makeCore({ show }));
    const terms = [...container.querySelectorAll("dt")].map((d) => d.textContent?.trim());
    const at = (name: string) => terms.indexOf(name);
    expect(terms.slice(at("ステータス"), at("ステータス") + 7)).toEqual([
      "ステータス",
      "優先度",
      "開始日",
      "期限",
      "見積もり",
      "ポイント",
      "ラベル",
    ]);
    expect(at("担当者")).toBeGreaterThan(at("ラベル"));
    expect(at("area")).toBeGreaterThan(at("ラベル"));
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
    // a built-in property is a field of the task, a property of the schema is in props
    await fireEvent.click(screen.getByRole("button", { name: "優先度" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "high" }));
    expect(k.save.edit).toHaveBeenCalledWith("U6", "shared", "priority", "high");
    await fireEvent.change(screen.getByLabelText("期限"), { target: { value: "2026-11-09" } });
    expect(k.save.edit).toHaveBeenCalledWith("U6", "shared", "due", "2026-11-09");
    await fireEvent.change(screen.getByLabelText("kickoff"), { target: { value: "2026-11-02" } });
    expect(k.save.edit).toHaveBeenCalledWith("U6", "shared", "props.kickoff", "2026-11-02");
  });

  it("clears a date with null, and the priority with null", async () => {
    const k = makeCore({ show });
    mount(PropsPanel, { show }, k);
    await fireEvent.change(screen.getByLabelText("期限"), { target: { value: "" } });
    expect(k.save.edit).toHaveBeenCalledWith("U6", "shared", "due", null);
    await fireEvent.click(screen.getByRole("button", { name: "優先度" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "—" }));
    expect(k.save.edit).toHaveBeenCalledWith("U6", "shared", "priority", null);
  });

  it("chooses the priority from the priorities of the schema", async () => {
    mount(PropsPanel, { show }, makeCore({ show }));
    await fireEvent.click(screen.getByRole("button", { name: "優先度" }));
    expect(screen.getAllByRole("menuitem").map((i) => i.textContent?.trim())).toEqual([
      "—",
      "low",
      "mid",
      "high",
    ]);
  });

  it("closes the status menu when C opens the create dialog over it", async () => {
    const k = makeCore({ show });
    mount(PropsPanel, { show }, k);
    mount(CreateHost, {}, k);
    await fireEvent.click(screen.getByRole("button", { name: "ステータス" }));
    const item = screen.getByRole("menuitem", { name: "完了 · done" });
    item.focus();
    await fireEvent.keyDown(item, { key: "c" });
    expect(await screen.findByRole("dialog")).toHaveTextContent("タスクを作成");
    await waitFor(() => expect(screen.queryByRole("menu")).toBeNull());
  });

  it("edits a number while typing and commits on blur", async () => {
    const k = makeCore({ show });
    mount(PropsPanel, { show }, k);
    const points = screen.getByRole("spinbutton", { name: "ポイント" });
    await fireEvent.input(points, { target: { value: "8" } });
    expect(k.save.edit).toHaveBeenCalledWith("U6", "shared", "points", 8);
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
    // a label is chosen from the labels of the schema, not typed
    expect(screen.queryByRole("textbox", { name: "ラベル" })).toBeNull();
    await fireEvent.change(screen.getByRole("combobox", { name: "ラベル" }), {
      target: { value: "ux" },
    });
    expect(k.save.edit).toHaveBeenCalledWith("U6", "shared", "labels", ["bug", "ux"]);
    expect(k.save.flush).toHaveBeenCalledWith("U6");
  });

  it("points to the settings when the schema lists no label", () => {
    const k = makeCore({ show: { ...show, labels: [] } });
    k.schema.get.mockImplementation(() => ({ ...schemaView, labels: [] }) as never);
    mount(PropsPanel, { show: { ...show, labels: [] } }, k);
    expect(screen.getByRole("link", { name: "設定の「ラベル」で足してください" })).toHaveAttribute(
      "href",
      "/settings/schema/shared?tab=labels",
    );
  });

  it("does not offer a kind the parent cannot hold", async () => {
    const child = showView({ ...show, parent: "UP" });
    const k = makeCore({ show: child });
    k.cache.task.mockImplementation((u: string) =>
      u === "UP" ? ({ ulid: "UP", kind: "task" } as never) : undefined,
    );
    mount(PropsPanel, { show: child }, k);
    await fireEvent.click(screen.getByRole("button", { name: "種類" }));
    expect(screen.getByRole("menuitem", { name: /^Project/ })).toBeDisabled();
  });

  it("takes the kind of the parent the screen read when the cache lacks the parent", async () => {
    const child = showView({ ...show, parent: "UP" });
    mount(PropsPanel, { show: child, parentKind: "task" }, makeCore({ show: child }));
    await fireEvent.click(screen.getByRole("button", { name: "種類" }));
    expect(screen.getByRole("menuitem", { name: /^Project/ })).toBeDisabled();
  });

  it("rules no kind out while the kind of the parent is not known", async () => {
    const child = showView({ ...show, parent: "UP" });
    mount(PropsPanel, { show: child }, makeCore({ show: child }));
    await fireEvent.click(screen.getByRole("button", { name: "種類" }));
    expect(screen.getByRole("menuitem", { name: /^Project/ })).not.toBeDisabled();
  });

  it("does not edit when read-only", () => {
    mount(PropsPanel, { show, readonly: true }, makeCore({ show }));
    expect(screen.getByRole("button", { name: "ステータス" })).toBeDisabled();
    expect(screen.getByLabelText("期限")).toHaveAttribute("readonly");
    expect(screen.getByRole("button", { name: "優先度" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "bug を外す" })).toBeNull();
    expect(screen.queryByRole("combobox", { name: "ラベル" })).toBeNull();
  });

  it("lists the watchers", () => {
    mount(PropsPanel, { show }, makeCore({ show }));
    expect(screen.getByRole("list", { name: "ウォッチャー" })).toHaveTextContent("koki");
  });
});
