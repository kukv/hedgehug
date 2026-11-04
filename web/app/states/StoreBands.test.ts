import { cleanup, screen, waitFor } from "@testing-library/svelte";
import { afterEach, describe, expect, it } from "vitest";
import { renderWithCore } from "../test-utils";
import StoreBands from "./StoreBands.svelte";

afterEach(cleanup);

const withReadOnly = (why: string | null) => ({
  stores: {
    readOnly: (s: string) => (s === "ro" ? why : null),
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
});
