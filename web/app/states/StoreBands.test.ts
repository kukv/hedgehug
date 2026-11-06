import { cleanup, fireEvent, screen, waitFor } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { toApiError } from "../../core/api/client";
import { renderWithCore } from "../test-utils";
import { resetToasts, toasts } from "../toast.svelte";
import StoreBands from "./StoreBands.svelte";

afterEach(cleanup);

const withReadOnly = (why: string | null) => ({
  stores: {
    readOnly: (s: string) => (s === "ro" ? why : null),
    hidden: () => false,
    subscribe: () => () => {},
  },
});

describe("StoreBands", () => {
  it("shows the band with the reason in Japanese for a read-only Store", () => {
    renderWithCore(StoreBands, { store: "ro" }, withReadOnly("the remote cannot be written"));
    expect(screen.getByRole("status", { name: "閲覧のみ" })).toHaveTextContent(
      "この Store は閲覧のみです（remote に書き込めません）",
    );
  });

  it("follows the directory: the band appears and goes when a sync changes the state", async () => {
    const fns = new Set<() => void>();
    let why: string | null = null;
    renderWithCore(
      StoreBands,
      { store: "ro" },
      {
        stores: {
          readOnly: () => why,
          hidden: () => false,
          subscribe: (fn: () => void) => {
            fns.add(fn);
            return () => fns.delete(fn);
          },
        },
      },
    );
    expect(screen.queryByRole("status")).toBeNull();
    why = "the remote cannot be written";
    for (const f of fns) f();
    expect(await screen.findByRole("status", { name: "閲覧のみ" })).toBeInTheDocument();
    why = null;
    for (const f of fns) f();
    await waitFor(() => expect(screen.queryByRole("status")).toBeNull());
  });

  it("shows nothing for a Store that can be written", () => {
    renderWithCore(StoreBands, { store: "demo" }, withReadOnly("the remote cannot be written"));
    expect(screen.queryByRole("status")).toBeNull();
  });

  describe("hidden Store", () => {
    beforeEach(resetToasts);

    function hiddenCore(patch: () => Promise<unknown>) {
      const reload = vi.fn(async () => {});
      const updateStore = vi.fn(patch);
      const over = {
        api: { updateStore },
        stores: {
          readOnly: () => null,
          hidden: (s: string) => s === "second",
          reload,
          subscribe: () => () => {},
        },
      };
      return { updateStore, reload, over };
    }
    const okPatch = async () => ({});

    it("says the Store is hidden, with the button to show it", () => {
      const { over } = hiddenCore(okPatch);
      renderWithCore(StoreBands, { store: "second" }, over);
      expect(screen.getByRole("status", { name: "非表示" })).toHaveTextContent(
        "この Store は非表示です",
      );
      expect(screen.getByRole("button", { name: "表示する" })).toBeEnabled();
      expect(screen.getByRole("button", { name: "表示する" })).toHaveAttribute(
        "aria-busy",
        "false",
      );
    });

    it("shows nothing for a Store that is shown", () => {
      const { over } = hiddenCore(okPatch);
      renderWithCore(StoreBands, { store: "first" }, over);
      expect(screen.queryByRole("status")).toBeNull();
    });

    it("shows the Store: updateStore hidden false, then reload and the toast", async () => {
      const { over, updateStore, reload } = hiddenCore(okPatch);
      renderWithCore(StoreBands, { store: "second" }, over);
      await fireEvent.click(screen.getByRole("button", { name: "表示する" }));
      await waitFor(() => expect(toasts.list[0]?.message).toBe("second を表示しました"));
      expect(updateStore).toHaveBeenCalledWith("second", { hidden: false });
      expect(reload).toHaveBeenCalled();
    });

    it("says why when the server refuses, and does not reload", async () => {
      const { over, reload } = hiddenCore(async () => {
        throw toApiError(404, { type: "store.not_found", title: "Not found", status: 404 });
      });
      renderWithCore(StoreBands, { store: "second" }, over);
      await fireEvent.click(screen.getByRole("button", { name: "表示する" }));
      await waitFor(() => expect(toasts.list).toHaveLength(1));
      expect(toasts.list[0]?.message).not.toBe("second を表示しました");
      expect(reload).not.toHaveBeenCalled();
    });

    it("stacks with the read-only band", () => {
      const { over } = hiddenCore(okPatch);
      over.stores.readOnly = () => "the remote cannot be written" as never;
      renderWithCore(StoreBands, { store: "second" }, over);
      expect(screen.getByRole("status", { name: "閲覧のみ" })).toBeInTheDocument();
      expect(screen.getByRole("status", { name: "非表示" })).toBeInTheDocument();
    });
  });
});
