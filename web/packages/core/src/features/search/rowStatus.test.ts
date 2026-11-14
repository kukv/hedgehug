import { describe, expect, it, vi } from "vitest";
import { Refused } from "../../model/errors";
import type { Item } from "../../model/item";
import { sprintSaveFailed } from "../planning/sprintDialogs";
import { setRowStatus } from "./rowStatus";

function deps(setProperty: () => Promise<Item>) {
  return {
    items: { setProperty: vi.fn(setProperty) },
    toasts: { show: vi.fn() },
    reload: vi.fn(async () => {}),
  };
}

describe("setRowStatus", () => {
  it("saves the status and reloads", async () => {
    const d = deps(async () => ({}) as Item);
    await setRowStatus(d, "S1", "I1", "st2");
    expect(d.items.setProperty).toHaveBeenCalledWith("S1", "I1", "status", "st2");
    expect(d.toasts.show).not.toHaveBeenCalled();
    expect(d.reload).toHaveBeenCalledOnce();
  });

  it("shows the refusal as an error toast and reloads", async () => {
    const d = deps(async () => {
      throw new Refused("閲覧のみの Store です");
    });
    await setRowStatus(d, "S1", "I1", "st2");
    expect(d.toasts.show).toHaveBeenCalledWith("閲覧のみの Store です", { error: true });
    expect(d.reload).toHaveBeenCalledOnce();
  });

  it("shows the failure text for another error and reloads", async () => {
    const d = deps(async () => {
      throw new Error("network");
    });
    await setRowStatus(d, "S1", "I1", "st2");
    expect(d.toasts.show).toHaveBeenCalledWith(sprintSaveFailed, { error: true });
    expect(d.reload).toHaveBeenCalledOnce();
  });
});
