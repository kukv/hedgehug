import { describe, expect, it } from "vitest";
import { item } from "./fixtures.test-util";
import {
  deletedValue,
  propertyRows,
  statusOptions,
  // biome-ignore lint/suspicious/noShadowRestrictedNames: 仕様の名前(アイテムの値を読む)
  valueOf,
  visibleProperties,
} from "./properties";
import { settings } from "./settings.test-util";

const task = {
  ...item("S1", "S1-P1", 2, "Task", "S1-I1"),
  status: "st2",
  priority: "gone",
  assignee: "taro",
  estimate: 1.5,
  systems: ["sy2"],
  choices: { F1: ["C2"] },
  fields: { F2: "01CHOICE00000000000000000A" },
};
const people = {
  members: [
    { login: "taro", name: "太郎" },
    { login: "hanako", name: "花子" },
  ],
  me: "hanako",
};
const byId = (id: string) => {
  const p = settings.properties.find((x) => x.id === id);
  if (!p) throw new Error(id);
  return p;
};

describe("properties", () => {
  it("shows visible properties for the kind in settings order", () => {
    expect(visibleProperties(settings, "Task").map((p) => p.id)).toEqual([
      "status",
      "assignee",
      "priority",
      "start",
      "estimate",
      "systems",
      "F1",
      "F2",
    ]);
    expect(visibleProperties(settings, "Story").map((p) => p.id)).toContain("points");
  });

  it("reads values by type, an ID-shaped text as text", () => {
    expect(valueOf(task, byId("estimate"))).toBe("1.5");
    expect(valueOf(task, byId("systems"))).toEqual(["sy2"]);
    expect(valueOf(task, byId("F1"))).toBe("C2");
    expect(valueOf(task, byId("F2"))).toBe("01CHOICE00000000000000000A");
  });

  it("labels values and marks deleted ones", () => {
    const rows = propertyRows(settings, "Task", (p) => valueOf(task, p), people);
    const row = (id: string) => {
      const r = rows.find((x) => x.property.id === id);
      if (!r) throw new Error(id);
      return r;
    };
    expect(row("status").text).toBe("InProgress");
    expect(row("priority")).toMatchObject({ text: deletedValue, deleted: true });
    expect(row("systems").text).toBe("API");
    expect(row("F1")).toMatchObject({ text: "検証", extra: true });
    expect(row("F1").options[0]).toMatchObject({ id: "", label: "なし" });
    expect(row("assignee").options.map((o) => o.label)).toEqual([
      "未割り当て",
      "太郎",
      "花子(あなた)",
    ]);
    expect(row("assignee").text).toBe("太郎");
  });

  it("lists statuses with their category and checks the current one", () => {
    expect(statusOptions(settings, "st2").slice(0, 2)).toEqual([
      { id: "st1", label: "Todo", hint: "未着手", checked: false },
      { id: "st2", label: "InProgress", hint: "進行中", checked: true },
    ]);
  });

  const rowsFor = (over: object) => {
    const it = { ...task, ...over };
    return propertyRows(settings, "Task", (p) => valueOf(it, p), people);
  };
  const rowOf = (rows: ReturnType<typeof rowsFor>, id: string) => {
    const r = rows.find((x) => x.property.id === id);
    if (!r) throw new Error(id);
    return r;
  };

  it("shows a login that is not a member as an unknown user, checked in the options", () => {
    const row = rowOf(rowsFor({ assignee: "ghost" }), "assignee");
    expect(row.text).toBe("不明なユーザー");
    expect(row.options.find((o) => o.checked)).toEqual({
      id: "ghost",
      label: "不明なユーザー",
      checked: true,
    });
    expect(row.options.map((o) => o.label)).toContain("太郎");
  });

  it("matches logins case-insensitively for the label and the check", () => {
    const row = rowOf(rowsFor({ assignee: "HANAKO" }), "assignee");
    expect(row.text).toBe("花子(あなた)");
    expect(row.options.filter((o) => o.checked).map((o) => o.id)).toEqual(["hanako"]);
  });

  it("shows an empty assignee as unassigned and an empty status as deleted", () => {
    expect(rowOf(rowsFor({ assignee: "" }), "assignee").text).toBe("未割り当て");
    expect(rowOf(rowsFor({ status: "" }), "status")).toMatchObject({
      text: deletedValue,
      deleted: true,
    });
  });
});
