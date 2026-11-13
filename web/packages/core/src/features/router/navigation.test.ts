import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { fakeHistory } from "./history.test-util";
import { createNavigation } from "./navigation";

describe("navigation", () => {
  it("starts at the route of the current address", () => {
    const nav = createNavigation(fakeHistory("/s/work/i/HH-3").history);
    expect(get(nav.route)).toEqual({ name: "item", store: "work", item: "HH-3" });
  });

  it("pushes on go, replaces on replace, and follows the back button", () => {
    const { history, stack, back } = fakeHistory("/");
    const nav = createNavigation(history);
    const stop = nav.start();
    nav.go({ name: "project", store: "work", project: "HH-P1", tab: "overview" });
    nav.replace({ name: "project", store: "work", project: "HH-P1", tab: "tree" });
    expect(stack).toEqual(["/", "/s/work/p/HH-P1/tree"]);
    expect(get(nav.route)).toMatchObject({ name: "project", tab: "tree" });
    back();
    expect(get(nav.route)).toEqual({ name: "home" });
    stop();
  });

  it("does not push the address that is already open", () => {
    const { history, stack } = fakeHistory("/");
    const nav = createNavigation(history);
    nav.go({ name: "workspace", store: "work", prefix: "HH", tab: "overview" });
    nav.go({ name: "workspace", store: "work", prefix: "HH", tab: "overview" });
    expect(stack).toEqual(["/", "/s/work/w/HH/overview"]);
  });

  it("stops following the back button", () => {
    const { history, back } = fakeHistory("/");
    const nav = createNavigation(history);
    nav.go({ name: "workspace", store: "work", prefix: "HH", tab: "overview" });
    nav.start()();
    back();
    expect(get(nav.route)).toEqual({
      name: "workspace",
      store: "work",
      prefix: "HH",
      tab: "overview",
    });
  });

  it("hands the section to open only once", () => {
    const nav = createNavigation(fakeHistory("/").history);
    nav.go(
      { name: "project", store: "work", project: "HH-P1", tab: "settings" },
      "rules-templates",
    );
    expect(nav.takeSection()).toBe("rules-templates");
    expect(nav.takeSection()).toBeNull();
  });

  it("forgets a section when the next move names none", () => {
    const nav = createNavigation(fakeHistory("/").history);
    nav.go(
      { name: "project", store: "work", project: "HH-P1", tab: "settings" },
      "rules-templates",
    );
    nav.go({ name: "home" });
    expect(nav.takeSection()).toBeNull();
  });

  it("forgets a section on replace", () => {
    const nav = createNavigation(fakeHistory("/").history);
    nav.go(
      { name: "project", store: "work", project: "HH-P1", tab: "settings" },
      "rules-templates",
    );
    nav.replace({ name: "project", store: "work", project: "HH-P1", tab: "tree" });
    expect(nav.takeSection()).toBeNull();
  });

  it("forgets a section on back", () => {
    const { history, back } = fakeHistory("/");
    const nav = createNavigation(history);
    nav.start();
    nav.go(
      { name: "project", store: "work", project: "HH-P1", tab: "settings" },
      "rules-templates",
    );
    back();
    expect(nav.takeSection()).toBeNull();
  });
});
