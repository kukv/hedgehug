import { describe, expect, it } from "vitest";
import type { HistoryPort } from "../../port/history";
import { get } from "../../reactive/store";
import { createNavigation } from "./navigation";

// fakeHistory は、積んだパスを持つ履歴の偽物。back は、ブラウザの戻るを押したときの動き。
function fakeHistory(start: string) {
  const stack = [start];
  let index = 0;
  const listeners = new Set<() => void>();
  const history: HistoryPort = {
    path: () => stack[index] ?? "/",
    push(path) {
      stack.splice(index + 1, stack.length, path);
      index = stack.length - 1;
    },
    replace(path) {
      stack[index] = path;
    },
    onPop(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
  const back = () => {
    index -= 1;
    for (const l of listeners) l();
  };
  return { history, stack, back };
}

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
    nav.go({ name: "workspace", store: "work", prefix: "HH" });
    nav.go({ name: "workspace", store: "work", prefix: "HH" });
    expect(stack).toEqual(["/", "/s/work/w/HH"]);
  });

  it("stops following the back button", () => {
    const { history, back } = fakeHistory("/");
    const nav = createNavigation(history);
    nav.go({ name: "workspace", store: "work", prefix: "HH" });
    nav.start()();
    back();
    expect(get(nav.route)).toEqual({ name: "workspace", store: "work", prefix: "HH" });
  });
});
