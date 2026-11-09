import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import LabelPicker from "./LabelPicker.svelte";

const base = { store: "shared", label: "ラベル", options: ["bug", "ux", "ops"] };

describe("LabelPicker", () => {
  it("shows the labels the task holds, and adds one from the list of the schema only", async () => {
    const onedit = vi.fn();
    const oncommit = vi.fn();
    render(LabelPicker, { ...base, value: ["bug"], onedit, oncommit });
    expect(screen.getByText("bug")).toBeTruthy();
    const pick = screen.getByRole("combobox", { name: "ラベル" }) as HTMLSelectElement;
    // what the task holds is not offered again; there is no input for a free text
    expect([...pick.options].map((o) => o.value)).toEqual(["", "ux", "ops"]);
    expect(screen.queryByRole("textbox")).toBeNull();
    await fireEvent.change(pick, { target: { value: "ops" } });
    expect(onedit).toHaveBeenCalledWith(["bug", "ops"]);
    expect(oncommit).toHaveBeenCalledTimes(1);
  });

  it("takes a label off", async () => {
    const onedit = vi.fn();
    const oncommit = vi.fn();
    render(LabelPicker, { ...base, value: ["bug", "ux"], onedit, oncommit });
    await fireEvent.click(screen.getByRole("button", { name: "bug を外す" }));
    expect(onedit).toHaveBeenCalledWith(["ux"]);
    expect(oncommit).toHaveBeenCalledTimes(1);
  });

  it("points to the labels of the settings when the list is empty", () => {
    render(LabelPicker, { ...base, options: [], value: [], onedit: vi.fn(), oncommit: vi.fn() });
    const link = screen.getByRole("link", { name: "設定の「ラベル」で足してください" });
    expect(link.getAttribute("href")).toBe("/settings/schema/shared?tab=labels");
    expect(screen.queryByRole("combobox")).toBeNull();
  });

  it("still shows and takes off a label that the list lacks", async () => {
    const onedit = vi.fn();
    render(LabelPicker, { ...base, options: [], value: ["old"], onedit, oncommit: vi.fn() });
    await fireEvent.click(screen.getByRole("button", { name: "old を外す" }));
    expect(onedit).toHaveBeenCalledWith([]);
  });

  it("has nothing to press when it only reads, and says nothing about the settings", () => {
    render(LabelPicker, {
      ...base,
      options: [],
      value: ["bug"],
      readonly: true,
      onedit: vi.fn(),
      oncommit: vi.fn(),
    });
    expect(screen.getByText("bug")).toBeTruthy();
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.queryByRole("combobox")).toBeNull();
    expect(screen.queryByRole("link")).toBeNull();
  });

  it("offers nothing more when the task holds every label of the list", () => {
    render(LabelPicker, {
      ...base,
      value: ["bug", "ux", "ops"],
      onedit: vi.fn(),
      oncommit: vi.fn(),
    });
    expect(screen.queryByRole("combobox")).toBeNull();
    expect(screen.queryByRole("link")).toBeNull();
  });

  it("tells a label from another without regard to case: a task that holds Bug is not offered bug", () => {
    render(LabelPicker, { ...base, value: ["Bug"], onedit: vi.fn(), oncommit: vi.fn() });
    const pick = screen.getByRole("combobox", { name: "ラベル" }) as HTMLSelectElement;
    expect([...pick.options].map((o) => o.value)).toEqual(["", "ux", "ops"]);
  });

  it("shows the labels the task holds while the schema is not read, and offers nothing", () => {
    render(LabelPicker, {
      ...base,
      options: undefined,
      value: ["bug"],
      onedit: vi.fn(),
      oncommit: vi.fn(),
    });
    expect(screen.getByText("bug")).toBeTruthy();
    expect(screen.queryByRole("combobox")).toBeNull();
    expect(screen.queryByRole("link")).toBeNull();
  });

  it("points the group at the text that says why it only reads", () => {
    render(LabelPicker, {
      ...base,
      value: ["bug"],
      readonly: true,
      describedby: "why",
      onedit: vi.fn(),
      oncommit: vi.fn(),
    });
    expect(screen.getByRole("group", { name: "ラベル" })).toHaveAttribute(
      "aria-describedby",
      "why",
    );
  });

  it("tells the caller before the link to the settings is followed", async () => {
    const onnavigate = vi.fn();
    render(LabelPicker, {
      ...base,
      options: [],
      value: [],
      onnavigate,
      onedit: vi.fn(),
      oncommit: vi.fn(),
    });
    await fireEvent.click(screen.getByRole("link", { name: "設定の「ラベル」で足してください" }));
    expect(onnavigate).toHaveBeenCalledTimes(1);
  });
});
