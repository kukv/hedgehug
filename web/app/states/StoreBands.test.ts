import { cleanup, screen } from "@testing-library/svelte";
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

  it("shows nothing for a Store that can be written", () => {
    renderWithCore(StoreBands, { store: "demo" }, withReadOnly("the remote cannot be written"));
    expect(screen.queryByRole("status")).toBeNull();
  });
});
