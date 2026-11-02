import { afterEach, describe, expect, it, vi } from "vitest";
import { callAll } from "./listeners";

describe("callAll", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("runs the next listener after one throws, and reports the error once", () => {
    const err = vi.spyOn(console, "error").mockImplementation(() => {});
    const boom = new Error("listener");
    const next = vi.fn();
    callAll([
      () => {
        throw boom;
      },
      next,
    ]);
    expect(next).toHaveBeenCalledTimes(1);
    expect(err).toHaveBeenCalledTimes(1);
    expect(err).toHaveBeenCalledWith(boom);
  });
});
