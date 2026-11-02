import { fireEvent, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import TaskPicker from "./TaskPicker.svelte";
import { makeCore, mount, ok, taskView } from "./testkit";

describe("TaskPicker", () => {
  it("searches the store by the typed words and picks a result with a click", async () => {
    const k = makeCore();
    k.api.GET.mockImplementation(() =>
      ok({
        tasks: [
          taskView({ ulid: "U1", id: "REL-8", title: "リリース作業" }),
          taskView({ ulid: "U2", id: "REL-9", title: "別の作業" }),
        ],
      }),
    );
    const onpick = vi.fn();
    mount(TaskPicker, { store: "shared", debounceMs: 0, onpick }, k);
    await fireEvent.input(screen.getByRole("combobox", { name: "タスクを検索" }), {
      target: { value: "REL" },
    });
    const option = await screen.findByRole("option", { name: /REL-8/ });
    expect(k.api.GET).toHaveBeenCalledWith("/api/tasks", {
      params: { query: { q: '"REL"', store: "shared" } },
    });
    await fireEvent.click(option);
    expect(onpick).toHaveBeenCalledWith(expect.objectContaining({ ulid: "U1" }));
  });

  it("leaves out excluded tasks and picks the active one with Enter", async () => {
    const k = makeCore();
    k.api.GET.mockImplementation(() =>
      ok({
        tasks: [
          taskView({ ulid: "U1", id: "REL-1", title: "self" }),
          taskView({ ulid: "U2", id: "REL-2", title: "other" }),
        ],
      }),
    );
    const onpick = vi.fn();
    mount(TaskPicker, { store: "shared", exclude: ["U1"], debounceMs: 0, onpick }, k);
    const input = screen.getByRole("combobox", { name: "タスクを検索" });
    await fireEvent.input(input, { target: { value: "REL" } });
    await screen.findByRole("option", { name: /REL-2/ });
    expect(screen.queryByRole("option", { name: /REL-1/ })).toBeNull();
    await fireEvent.keyDown(input, { key: "Enter" });
    expect(onpick).toHaveBeenCalledWith(expect.objectContaining({ ulid: "U2" }));
  });

  it("ignores Enter and the arrows while an IME is converting", async () => {
    const k = makeCore();
    k.api.GET.mockImplementation(() =>
      ok({ tasks: [taskView({ ulid: "U1", id: "REL-1", title: "a" })] }),
    );
    const onpick = vi.fn();
    mount(TaskPicker, { store: "shared", debounceMs: 0, onpick }, k);
    const input = screen.getByRole("combobox", { name: "タスクを検索" });
    await fireEvent.input(input, { target: { value: "REL" } });
    await screen.findByRole("option", { name: /REL-1/ });
    await fireEvent.keyDown(input, { key: "Enter", isComposing: true });
    await fireEvent.keyDown(input, { key: "Enter", keyCode: 229 });
    expect(onpick).not.toHaveBeenCalled();
    await fireEvent.keyDown(input, { key: "Enter" });
    expect(onpick).toHaveBeenCalledTimes(1);
  });
});
