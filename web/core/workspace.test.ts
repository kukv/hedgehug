import { describe, expect, it } from "vitest";
import {
  detachedText,
  NAME_RULE,
  nameProblem,
  PREFIX_RULE,
  prefixDay,
  prefixProblem,
  removeBlocker,
} from "./workspace";

const all = [
  { store: "shared", ulid: "W1", name: "release", prefix: "REL" },
  { store: "shared", ulid: "W2", name: "shared", prefix: "SH" },
  { store: "backend", ulid: "B1", name: "api", prefix: "API" },
];

describe("nameProblem", () => {
  it("accepts a free name", () => {
    expect(nameProblem("ops", "shared", all)).toBeNull();
    expect(nameProblem("ops-2_b", "shared", all)).toBeNull();
  });

  it("asks for a name, and refuses what the server refuses", () => {
    expect(nameProblem("", "shared", all)).toBe("名前を入れてください");
    for (const bad of ["Ops", "-ops", "o ps", "運用"]) {
      expect(nameProblem(bad, "shared", all)).toBe(NAME_RULE);
    }
  });

  it("refuses the name of a workspace of the same store, but not of another store", () => {
    expect(nameProblem("shared", "shared", all)).toBe(
      "shared は同じ Store のワークスペースが使っています",
    );
    expect(nameProblem("api", "shared", all)).toBeNull();
  });

  it("does not count the workspace that is renamed", () => {
    expect(nameProblem("release", "shared", all, "W1")).toBeNull();
    expect(nameProblem("shared", "shared", all, "W1")).not.toBeNull();
  });
});

describe("prefixProblem", () => {
  it("accepts a free prefix", () => {
    expect(prefixProblem("OPS", "shared", all)).toBeNull();
    expect(prefixProblem("A1", "shared", all)).toBeNull();
  });

  it("asks for a prefix, and refuses what the server refuses", () => {
    expect(prefixProblem("", "shared", all)).toBe("接頭辞を入れてください");
    for (const bad of ["O", "ops", "1OP", "ABCDEFGHIJK", "OP-S"]) {
      expect(prefixProblem(bad, "shared", all)).toBe(PREFIX_RULE);
    }
  });

  it("says which workspace has the prefix, in the same store or in another", () => {
    expect(prefixProblem("SH", "shared", all)).toBe(
      "SH は同じ Store のワークスペース shared が使っています",
    );
    expect(prefixProblem("API", "shared", all)).toBe(
      "API はマウント中の backend のワークスペース api が使っています。別の接頭辞にしてください",
    );
  });

  it("does not count the workspace whose prefix is changed", () => {
    expect(prefixProblem("REL", "shared", all, "W1")).toBeNull();
  });
});

describe("removeBlocker", () => {
  it("lets an empty workspace go", () => {
    expect(removeBlocker({ tasks: 0, deleted: 0 })).toBeNull();
  });

  it("counts the deleted tasks, which still hold the workspace", () => {
    expect(removeBlocker({ tasks: 24, deleted: 1 })).toBe(
      "タスク 25 件（削除済み 1 件を含む）が残っているため削除できません。別のワークスペースへ移すか完全に削除すると消せます",
    );
    expect(removeBlocker({ tasks: 2, deleted: 0 })).toBe(
      "タスク 2 件が残っているため削除できません。別のワークスペースへ移すか完全に削除すると消せます",
    );
    expect(removeBlocker({ tasks: 0, deleted: 3 })).toContain("タスク 3 件（削除済み 3 件を含む）");
  });
});

describe("detachedText", () => {
  it("says nothing when nothing was taken off", () => {
    expect(detachedText(null)).toBe("");
    expect(detachedText([])).toBe("");
    expect(detachedText([{ task: "U1", id: "SH-1" }])).toBe("");
  });

  it("names the sprint and the releases of each task", () => {
    expect(
      detachedText([
        { task: "U1", id: "SH-12", sprint: "Sprint 44", releases: ["v2.4", "v2.5"] },
        { task: "U2", id: "SH-13", releases: ["v2.4"] },
      ]),
    ).toBe(
      "SH-12 はスプリント Sprint 44 とリリース v2.4、v2.5 から外れました。SH-13 はリリース v2.4 から外れました",
    );
  });
});

describe("prefixDay", () => {
  it("shows the day the prefix was set, and a dash for the zero time or nothing", () => {
    const now = new Date(2026, 10, 3);
    expect(prefixDay("2026-10-29T12:00:00Z", now)).toBe("10/29");
    expect(prefixDay("0001-01-01T00:00:00Z", now)).toBe("—");
    expect(prefixDay("", now)).toBe("—");
  });
});
