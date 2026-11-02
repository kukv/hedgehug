import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { dismissToast, resetToasts, toast, toasts, undoToast } from "./toast.svelte";

beforeEach(() => {
  vi.useFakeTimers();
  resetToasts();
});
afterEach(() => {
  vi.useRealTimers();
});

describe("toast", () => {
  it("取り消しなしは 6 秒で消える", () => {
    toast({ message: "保存しました" });
    expect(toasts.list).toHaveLength(1);
    vi.advanceTimersByTime(5999);
    expect(toasts.list).toHaveLength(1);
    vi.advanceTimersByTime(1);
    expect(toasts.list).toHaveLength(0);
  });

  it("取り消し付きは 10 秒で消える", () => {
    toast({ message: "削除しました", undo: () => {} });
    vi.advanceTimersByTime(9999);
    expect(toasts.list).toHaveLength(1);
    vi.advanceTimersByTime(1);
    expect(toasts.list).toHaveLength(0);
  });

  it("undoToast は取り消しを呼んで閉じる", async () => {
    const undo = vi.fn();
    toast({ message: "削除しました", undo });
    await undoToast(toasts.list[0]?.id ?? 0);
    expect(undo).toHaveBeenCalledTimes(1);
    expect(toasts.list).toHaveLength(0);
  });

  it("取り消しが失敗したらエラーの文言をトーストに出す", async () => {
    toast({
      message: "削除しました",
      undo: () => {
        throw new Error("boom");
      },
    });
    await undoToast(toasts.list[0]?.id ?? 0);
    expect(toasts.list.map((t) => t.message)).toEqual(["操作に失敗しました: boom"]);
  });

  it("dismissToast は指定の1つだけ閉じる", () => {
    toast({ message: "a" });
    toast({ message: "b" });
    dismissToast(toasts.list[0]?.id ?? 0);
    expect(toasts.list.map((t) => t.message)).toEqual(["b"]);
  });
});
