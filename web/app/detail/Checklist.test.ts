import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import Checklist from "./Checklist.svelte";

const body = "intro\n- [x] 生成コード\n- [ ] 認証ヘッダ\n- [ ] ページング\n";

describe("Checklist", () => {
  it("shows the count and one checkbox per item", () => {
    render(Checklist, { body, ontoggle: vi.fn() });
    expect(screen.getByRole("heading", { name: /チェックリスト/ })).toHaveTextContent("1/3");
    expect(screen.getAllByRole("checkbox")).toHaveLength(3);
    expect(screen.getByRole("checkbox", { name: "生成コード" })).toBeChecked();
  });

  it("reports the line of the item that was toggled", async () => {
    const ontoggle = vi.fn();
    render(Checklist, { body, ontoggle });
    await fireEvent.click(screen.getByRole("checkbox", { name: "ページング" }));
    expect(ontoggle).toHaveBeenCalledWith(3);
  });

  it("renders nothing for a body without checkboxes, and disables them when read-only", () => {
    const { container, unmount } = render(Checklist, { body: "plain", ontoggle: vi.fn() });
    expect(container.querySelector("section")).toBeNull();
    unmount();
    render(Checklist, { body, readonly: true, ontoggle: vi.fn() });
    expect(screen.getByRole("checkbox", { name: "生成コード" })).toBeDisabled();
  });
});
