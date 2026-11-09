import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import { ApiError } from "../../core/api/client";
import { makeCore, mount, schemaView, taskView } from "../detail/testkit";
import RequiredDialog from "./RequiredDialog.svelte";

const t = taskView({
  ulid: "U16",
  id: "REL-16",
  title: "アラートの閾値を見直す",
  status: "in_progress",
});

function setup(
  over: { fields?: string[]; elsewhere?: string[]; onsubmit?: (b: unknown) => Promise<void> } = {},
) {
  const onsubmit = vi.fn(over.onsubmit ?? (async () => {}));
  const onclose = vi.fn();
  mount(
    RequiredDialog,
    {
      task: t,
      status: "done",
      fields: over.fields ?? ["points"],
      elsewhere: over.elsewhere ?? [],
      schema: schemaView,
      onsubmit,
      onclose,
    },
    makeCore(),
  );
  return { onsubmit, onclose };
}

describe("RequiredDialog", () => {
  it("says what the status needs, and keeps the button off until the field is filled", async () => {
    setup();
    expect(
      screen.getByRole("dialog", { name: "REL-16 を完了にするには ポイント が必要です" }),
    ).toBeInTheDocument();
    const go = screen.getByRole("button", { name: "完了にする" });
    expect(go).toBeDisabled();
    await fireEvent.input(screen.getByRole("spinbutton", { name: "ポイント" }), {
      target: { value: "3" },
    });
    expect(go).toBeEnabled();
    expect(screen.getByText(/hh set REL-16 status=done points=3/)).toBeInTheDocument();
  });

  it("sends the status and the fields in one call, then closes", async () => {
    const { onsubmit, onclose } = setup({ fields: ["points", "assignees"] });
    await fireEvent.input(screen.getByRole("spinbutton", { name: "ポイント" }), {
      target: { value: "3" },
    });
    expect(screen.getByRole("button", { name: "完了にする" })).toBeDisabled();
    const chips = screen.getByRole("textbox", { name: "担当者" });
    await fireEvent.input(chips, { target: { value: "github.com/koki" } });
    await fireEvent.keyDown(chips, { key: "Enter" });
    await fireEvent.click(screen.getByRole("button", { name: "完了にする" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(onsubmit).toHaveBeenCalledWith({
      status: "done",
      assignees: ["github.com/koki"],
      points: 3,
    });
  });

  it("sends a built-in property at the top of the body, the labels from the list of the schema, and a property of the schema in props", async () => {
    const { onsubmit, onclose } = setup({ fields: ["priority", "labels", "area", "due"] });
    expect(screen.getByRole("button", { name: "完了にする" })).toBeDisabled();
    // the priority is chosen from the priorities of the schema
    await fireEvent.click(screen.getByRole("button", { name: "優先度" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "high" }));
    // the labels are chosen from the labels of the schema, not typed
    expect(screen.queryByRole("textbox", { name: "ラベル" })).toBeNull();
    await fireEvent.change(screen.getByRole("combobox", { name: "ラベル" }), {
      target: { value: "bug" },
    });
    await fireEvent.input(screen.getByRole("textbox", { name: "area" }), {
      target: { value: "web" },
    });
    await fireEvent.change(screen.getByLabelText("期限"), { target: { value: "2026-11-07" } });
    expect(
      screen.getByText(/hh set REL-16 status=done priority=high labels=bug/),
    ).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("button", { name: "完了にする" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(onsubmit).toHaveBeenCalledWith({
      status: "done",
      priority: "high",
      labels: ["bug"],
      due: "2026-11-07",
      props: { area: "web" },
    });
  });

  it("keeps the dialog open and says why when the write is refused", async () => {
    const err = new ApiError({
      status: 422,
      type: "schema.violation",
      detail: "",
      args: { violations: [{ code: "required", field: "points", message: "x" }] },
    });
    const { onclose } = setup({
      onsubmit: async () => {
        throw err;
      },
    });
    await fireEvent.input(screen.getByRole("spinbutton", { name: "ポイント" }), {
      target: { value: "3" },
    });
    await fireEvent.click(screen.getByRole("button", { name: "完了にする" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("スキーマに合わない変更です");
    expect(onclose).not.toHaveBeenCalled();
  });

  it("closes on キャンセル without sending", async () => {
    const { onsubmit, onclose } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "キャンセル" }));
    expect(onclose).toHaveBeenCalled();
    expect(onsubmit).not.toHaveBeenCalled();
  });

  describe("with a field only the detail can fill", () => {
    it("lists it next to the ones the dialog takes in, with a link to the detail, and does not let the card move", async () => {
      const { onsubmit, onclose } = setup({
        fields: ["points"],
        elsewhere: ["parent", "watchers"],
      });
      expect(
        screen.getByRole("dialog", {
          name: "REL-16 を完了にするには ポイント・親・ウォッチャー が必要です",
        }),
      ).toBeInTheDocument();
      const links = screen.getAllByRole("link", { name: "詳細で入力" });
      expect(links).toHaveLength(2);
      expect(links[0]).toHaveAttribute("href", "/tasks/U16");
      await fireEvent.input(screen.getByRole("spinbutton", { name: "ポイント" }), {
        target: { value: "3" },
      });
      const go = screen.getByRole("button", { name: "完了にする" });
      expect(go).toBeDisabled();
      await fireEvent.click(go);
      expect(onsubmit).not.toHaveBeenCalled();
      expect(onclose).not.toHaveBeenCalled();
    });

    it("opens the detail and closes the dialog from the link", async () => {
      const { onclose } = setup({ fields: [], elsewhere: ["sprint"] });
      await fireEvent.click(screen.getByRole("link", { name: "詳細で入力" }));
      expect(onclose).toHaveBeenCalled();
      expect(window.location.pathname).toBe("/tasks/U16");
    });
  });
});
