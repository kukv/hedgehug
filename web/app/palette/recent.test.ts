import { describe, expect, it } from "vitest";
import { readRecent, rememberTask } from "./recent";

function memory(initial?: string): Storage {
  const m = new Map<string, string>(initial ? [["hedgehug.recent", initial]] : []);
  return {
    getItem: (k: string) => m.get(k) ?? null,
    setItem: (k: string, v: string) => void m.set(k, v),
  } as unknown as Storage;
}

describe("recent tasks", () => {
  it("puts the latest first, once, and keeps eight", () => {
    const s = memory();
    for (const u of ["a", "b", "c", "a", "d", "e", "f", "g", "h", "i"]) rememberTask(u, s);
    expect(readRecent(s)).toEqual(["i", "h", "g", "f", "e", "d", "a", "c"]);
  });

  it("is empty when nothing is stored, when the value is not a list, or when it cannot be read", () => {
    expect(readRecent(memory())).toEqual([]);
    expect(readRecent(memory("{}"))).toEqual([]);
    expect(readRecent(memory('[1,"a"]'))).toEqual(["a"]);
    expect(readRecent(memory("not json"))).toEqual([]);
    expect(
      readRecent({
        getItem() {
          throw new Error("blocked");
        },
      }),
    ).toEqual([]);
  });

  it("does not throw when the storage cannot be written", () => {
    const s = {
      getItem: () => null,
      setItem() {
        throw new Error("quota");
      },
    };
    expect(() => rememberTask("a", s)).not.toThrow();
  });
});
