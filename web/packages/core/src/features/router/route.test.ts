import { describe, expect, it } from "vitest";
import {
  formatRoute,
  itemDisplayId,
  parseDisplayId,
  parseRoute,
  projectDisplayId,
  type Route,
} from "./route";

describe("parseRoute and formatRoute", () => {
  const cases: [string, Route][] = [
    ["/", { name: "home" }],
    ["/s/work/w/HH/overview", { name: "workspace", store: "work", prefix: "HH", tab: "overview" }],
    ["/s/work/w/HH/settings", { name: "workspace", store: "work", prefix: "HH", tab: "settings" }],
    [
      "/s/work/p/HH-P1/overview",
      { name: "project", store: "work", project: "HH-P1", tab: "overview" },
    ],
    ["/s/work/p/HH-P1/tree", { name: "project", store: "work", project: "HH-P1", tab: "tree" }],
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
