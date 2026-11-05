import { describe, expect, it } from "vitest";
import type { SchemaView } from "./schema";
import {
  addValues,
  draftOf,
  type Lists,
  lockedStatuses,
  putBody,
  removeValue,
  requirableFields,
  type ScopedDraft,
  setShown,
  shownStatuses,
  violationQuery,
} from "./scoped";

const schema = {
  statuses: [
    { name: "todo", category: "todo", default: true },
    { name: "in_progress", category: "doing", default: true },
    { name: "waiting", category: "doing" },
    { name: "done", category: "done", default: true },
    { name: "canceled", category: "canceled", default: true },
  ],
  kinds: [],
  props: { points: { type: "number" }, area: { type: "text" } },
  scoped: [],
  workflow: { transitions: {}, required: {} },
  templates: [],
  templateDefaults: {},
} as SchemaView;

const none: ScopedDraft = {
  statuses: [],
  transitions: {},
  required: {},
  templates: [],
  templateDefaults: {},
};

describe("draftOf", () => {
  it("is empty for a project with no override", () => {
    expect(draftOf(undefined)).toEqual(none);
  });

  it("reads the parts of the override, a null list as empty", () => {
    expect(
      draftOf({
        where: "project:P1",
        statuses: ["todo", "done"],
        workflow: {
          transitions: { todo: ["done"], waiting: null },
          required: { done: ["points"] },
        },
      }),
    ).toEqual({
      statuses: ["todo", "done"],
      transitions: { todo: ["done"], waiting: [] },
      required: { done: ["points"] },
      templates: [],
      templateDefaults: {},
    });
  });
});

describe("shownStatuses and setShown", () => {
  it("shows every status of the store while the override lists none", () => {
    expect(shownStatuses(schema, none)).toEqual(schema.statuses?.map((s) => s.name));
  });

  it("hiding a status lists the others in the order of the schema", () => {
    const d = setShown(schema, none, "waiting", false);
    expect(d.statuses).toEqual(["todo", "in_progress", "done", "canceled"]);
    expect(shownStatuses(schema, d)).toEqual(d.statuses);
  });

  it("showing the last hidden status goes back to the store's (no list)", () => {
    const d = setShown(schema, setShown(schema, none, "waiting", false), "waiting", true);
    expect(d.statuses).toEqual([]);
  });

  it("does not hide the default of a category", () => {
    expect(setShown(schema, none, "todo", false)).toBe(none);
    expect([...lockedStatuses(schema)]).toEqual(["todo", "in_progress", "done", "canceled"]);
  });
});

describe("lists", () => {
  it("addValues adds to the values of a key without repeating one", () => {
    const l: Lists = { todo: ["done"] };
    expect(addValues(l, "todo", ["done", "waiting"])).toEqual({ todo: ["done", "waiting"] });
    expect(addValues(l, "waiting", ["done"])).toEqual({ todo: ["done"], waiting: ["done"] });
    expect(l).toEqual({ todo: ["done"] });
  });

  it("removeValue takes the key off with its last value", () => {
    expect(removeValue({ todo: ["done", "waiting"] }, "todo", "done")).toEqual({
      todo: ["waiting"],
    });
    expect(removeValue({ todo: ["done"], waiting: ["done"] }, "todo", "done")).toEqual({
      waiting: ["done"],
    });
  });
});

describe("requirableFields", () => {
  it("is the properties by name and the built-in fields the server accepts", () => {
    expect(requirableFields(schema)).toEqual([
      "area",
      "points",
      "parent",
      "assignees",
      "watchers",
      "labels",
      "repos",
      "blocked_by",
      "sprint",
      "releases",
      "links",
    ]);
  });
});

describe("putBody", () => {
  it("is empty while nothing changed", () => {
    expect(putBody(schema, none, none)).toEqual({});
    const saved = {
      statuses: ["todo"],
      transitions: { todo: ["done"] },
      required: {},
      templates: [],
      templateDefaults: {},
    };
    expect(putBody(schema, saved, structuredClone(saved))).toEqual({});
  });

  it("sends only the part that changed, as the whole of it", () => {
    const saved: ScopedDraft = {
      statuses: [],
      transitions: { todo: ["in_progress"] },
      required: { done: ["points"] },
      templates: [],
      templateDefaults: {},
    };
    const required = { done: ["points"], in_progress: ["area"] };
    expect(putBody(schema, saved, { ...saved, required })).toEqual({ required });
    const statuses = setShown(schema, saved, "waiting", false).statuses;
    expect(putBody(schema, saved, { ...saved, statuses })).toEqual({ statuses });
  });

  it("sends an empty part to go back to the store's", () => {
    const saved: ScopedDraft = {
      statuses: ["todo", "in_progress", "done", "canceled"],
      transitions: { todo: ["done"] },
      required: { done: ["points"] },
      templates: [],
      templateDefaults: {},
    };
    expect(putBody(schema, saved, none)).toEqual({
      statuses: [],
      transitions: {},
      required: {},
    });
  });

  it("does not take a list of every status for a change", () => {
    const all = schema.statuses?.map((s) => s.name) ?? [];
    expect(putBody(schema, { ...none, statuses: all }, none)).toEqual({});
  });

  it("does not take the order of the values for a change", () => {
    const saved: ScopedDraft = {
      ...none,
      required: { done: ["points", "area"] },
      templates: [],
      templateDefaults: {},
    };
    expect(
      putBody(schema, saved, {
        ...none,
        required: { done: ["area", "points"] },
        templates: [],
        templateDefaults: {},
      }),
    ).toEqual({});
  });
});

describe("violationQuery", () => {
  it("asks for the invalid tasks of the project by ULID", () => {
    expect(violationQuery("01ABC")).toBe("is:invalid project:01ABC");
  });
});
