import { describe, expect, it, vi } from "vitest";
import type { ScreenStatesPort } from "../../port/search";
import { createScreenStates } from "./screenStates";

const portOf = (body: string | null): ScreenStatesPort => ({
  get: vi.fn(async () => body),
  put: vi.fn(async () => {}),
});

describe("createScreenStates (E1-16)", () => {
  it("reads the server only on the first read of a screen", async () => {
    const port = portOf('{"query":"kind:bug"}');
    const s = createScreenStates(port);
    expect(await s.read("search", { query: "" })).toEqual({ query: "kind:bug" });
    expect(await s.read("search", { query: "" })).toEqual({ query: "kind:bug" });
    expect(port.get).toHaveBeenCalledTimes(1);
    expect(port.get).toHaveBeenCalledWith("search");
  });

  it("returns what was written, and puts it to the server as JSON", async () => {
    const port = portOf(null);
    const s = createScreenStates(port);
    s.write("search", { query: "a" });
    expect(await s.read("search", { query: "" })).toEqual({ query: "a" });
    expect(port.get).not.toHaveBeenCalled();
    expect(port.put).toHaveBeenCalledWith("search", '{"query":"a"}');
  });

  it("falls back when nothing is saved, the JSON is broken, or the read fails", async () => {
    expect(await createScreenStates(portOf(null)).read("search", { query: "x" })).toEqual({
      query: "x",
    });
    expect(await createScreenStates(portOf("{broken")).read("search", { query: "x" })).toEqual({
      query: "x",
    });
    const failing: ScreenStatesPort = {
      get: vi.fn(async () => {
        throw new Error("down");
      }),
      put: vi.fn(async () => {}),
    };
    expect(await createScreenStates(failing).read("search", { query: "x" })).toEqual({
      query: "x",
    });
  });

  it("keeps each screen apart, and a failed write does not throw", async () => {
    const port: ScreenStatesPort = {
      get: vi.fn(async () => null),
      put: vi.fn(async () => {
        throw new Error("down");
      }),
    };
    const s = createScreenStates(port);
    s.write("a", 1);
    await Promise.resolve();
    expect(await s.read("a", 0)).toBe(1);
    expect(await s.read("b", 0)).toBe(0);
  });

  it("shares one server read between reads made at the same time", async () => {
    const port = portOf('{"n":1}');
    const s = createScreenStates(port);
    await Promise.all([s.read("x", {}), s.read("x", {})]);
    expect(port.get).toHaveBeenCalledTimes(1);
  });
});
