import { describe, expect, it } from "vitest";
import { createCore } from "./core-factory";

describe("createCore", () => {
  it("7つの core を作る", () => {
    const c = createCore({
      token: () => "t",
      fetch: (() => new Promise(() => {})) as typeof fetch,
    });
    expect(Object.keys(c).sort()).toEqual([
      "api",
      "cache",
      "conn",
      "save",
      "schema",
      "stores",
      "views",
    ]);
    expect(typeof c.conn.start).toBe("function");
  });
});
