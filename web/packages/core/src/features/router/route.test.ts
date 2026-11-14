import { describe, expect, it } from "vitest";
import {
  formatRoute,
  itemDisplayId,
  parseDisplayId,
  parseRoute,
  projectDisplayId,
  projectTabOf,
  type Route,
  workspaceTabOf,
} from "./route";

describe("parseRoute and formatRoute", () => {
  const cases: [string, Route][] = [
    ["/", { name: "home" }],
    ["/s/work/w/HH/overview", { name: "workspace", store: "work", prefix: "HH", tab: "overview" }],
    ["/s/work/w/HH/settings", { name: "workspace", store: "work", prefix: "HH", tab: "settings" }],
    ["/s/work/w/HH/sprints", { name: "workspace", store: "work", prefix: "HH", tab: "sprints" }],
    ["/s/work/w/HH/releases", { name: "workspace", store: "work", prefix: "HH", tab: "releases" }],
    ["/s/work/w/HH/reports", { name: "workspace", store: "work", prefix: "HH", tab: "reports" }],
    [
      "/s/work/p/HH-P1/overview",
      { name: "project", store: "work", project: "HH-P1", tab: "overview" },
    ],
    ["/s/work/p/HH-P1/tree", { name: "project", store: "work", project: "HH-P1", tab: "tree" }],
    [
      "/s/work/p/HH-P1/settings",
      { name: "project", store: "work", project: "HH-P1", tab: "settings" },
    ],
    ["/s/work/i/HH-12", { name: "item", store: "work", item: "HH-12" }],
  ];

  it.each(cases)("reads %s and writes it back", (path, route) => {
    expect(parseRoute(path)).toEqual(route);
    expect(formatRoute(route)).toBe(path);
  });

  it("opens the overview when the workspace address has no tab", () => {
    expect(parseRoute("/s/work/w/HH")).toEqual({
      name: "workspace",
      store: "work",
      prefix: "HH",
      tab: "overview",
    });
  });

  it("sends an unknown workspace tab home", () => {
    expect(parseRoute("/s/work/w/HH/kanban")).toEqual({ name: "home" });
  });

  it("opens the overview when the project address has no tab", () => {
    expect(parseRoute("/s/work/p/HH-P1")).toEqual({
      name: "project",
      store: "work",
      project: "HH-P1",
      tab: "overview",
    });
  });

  it("turns a tab ID into a project tab, falling back to the overview", () => {
    expect(projectTabOf("settings")).toBe("settings");
    expect(projectTabOf("tree")).toBe("tree");
    expect(projectTabOf("overview")).toBe("overview");
    expect(projectTabOf("kanban")).toBe("overview");
  });

  it("turns a tab ID into a workspace tab, falling back to the overview", () => {
    expect(workspaceTabOf("sprints")).toBe("sprints");
    expect(workspaceTabOf("releases")).toBe("releases");
    expect(workspaceTabOf("reports")).toBe("reports");
    expect(workspaceTabOf("settings")).toBe("settings");
    expect(workspaceTabOf("overview")).toBe("overview");
    expect(workspaceTabOf("kanban")).toBe("overview");
  });

  it("encodes and decodes each part", () => {
    const route: Route = { name: "item", store: "a b", item: "x/y" };
    expect(formatRoute(route)).toBe("/s/a%20b/i/x%2Fy");
    expect(parseRoute("/s/a%20b/i/x%2Fy")).toEqual(route);
  });

  it.each([
    "/nope",
    "/s/work",
    "/s/work/x/HH-1",
    "/s/work/p/HH-P1/kanban",
    "/s/work/i/HH-1/more",
    "/s//i/HH-1",
    "/s/work/i/%E0%A4%A",
  ])("reads %s as home", (path) => {
    expect(parseRoute(path)).toEqual({ name: "home" });
  });
});

describe("display IDs", () => {
  it("reads item and project IDs", () => {
    expect(parseDisplayId("HH-12")).toEqual({ prefix: "HH", kind: "item", number: 12 });
    expect(parseDisplayId("WEB2-P3")).toEqual({ prefix: "WEB2", kind: "project", number: 3 });
  });

  it.each(["", "HH", "hh-1", "HH-0", "HH-P", "1H-1", "ABCDEFGHIJK-1", "HH-01", "HH-1x"])(
    "rejects %s",
    (s) => {
      expect(parseDisplayId(s)).toBeNull();
    },
  );

  it("writes item and project IDs", () => {
    expect(itemDisplayId("HH", 12)).toBe("HH-12");
    expect(projectDisplayId("HH", 1)).toBe("HH-P1");
  });
});

describe("settings", () => {
  it("reads /settings and /settings/accounts as the accounts tab", () => {
    expect(parseRoute("/settings")).toEqual({ name: "settings", tab: "accounts" });
    expect(parseRoute("/settings/accounts")).toEqual({ name: "settings", tab: "accounts" });
  });

  it("goes home for a tab that is not there", () => {
    expect(parseRoute("/settings/general")).toEqual({ name: "home" });
    expect(parseRoute("/settings/accounts/x")).toEqual({ name: "home" });
  });

  it("writes the settings address", () => {
    expect(formatRoute({ name: "settings", tab: "accounts" })).toBe("/settings/accounts");
  });
});

describe("inbox (06「受信箱」)", () => {
  it("reads /inbox and writes it back", () => {
    expect(parseRoute("/inbox")).toEqual({ name: "inbox" });
    expect(formatRoute({ name: "inbox" })).toBe("/inbox");
  });

  it("goes home for a path below /inbox", () => {
    expect(parseRoute("/inbox/mention")).toEqual({ name: "home" });
  });
});
