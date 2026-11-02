import { fireEvent, screen } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { closeCreate, openCreate } from "../create-state.svelte";
import { makeCore, mount } from "../detail/testkit";
import CreateHost from "./CreateHost.svelte";

describe("CreateHost", () => {
  beforeEach(() => history.replaceState(null, "", "/"));
  afterEach(() => closeCreate());

  it("opens the task dialog with C", async () => {
    mount(CreateHost, {}, makeCore());
    expect(screen.queryByRole("dialog")).toBeNull();
    await fireEvent.keyDown(window, { key: "c" });
    expect(await screen.findByRole("dialog")).toHaveTextContent("タスクを作成");
  });

  it("ignores C typed into an input, and C with a modifier", async () => {
    mount(CreateHost, {}, makeCore());
    const input = document.createElement("input");
    document.body.append(input);
    await fireEvent.keyDown(input, { key: "c" });
    await fireEvent.keyDown(window, { key: "c", ctrlKey: true });
    await fireEvent.keyDown(window, { key: "c", metaKey: true });
    expect(screen.queryByRole("dialog")).toBeNull();
    input.remove();
  });

  it("opens the project dialog for openCreate({ kind: 'project' })", async () => {
    mount(CreateHost, {}, makeCore());
    openCreate({ kind: "project" });
    expect(await screen.findByRole("dialog")).toHaveTextContent("プロジェクトを作成");
  });

  it("closes with the cancel button", async () => {
    mount(CreateHost, {}, makeCore());
    openCreate({ kind: "task" });
    await fireEvent.click(await screen.findByRole("button", { name: "キャンセル" }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("does not open a second dialog with C while one is open", async () => {
    mount(CreateHost, {}, makeCore());
    openCreate({ kind: "project" });
    await screen.findByRole("dialog");
    await fireEvent.keyDown(window, { key: "c" });
    expect(screen.getAllByRole("dialog")).toHaveLength(1);
    expect(screen.getByRole("dialog")).toHaveTextContent("プロジェクトを作成");
  });
});
