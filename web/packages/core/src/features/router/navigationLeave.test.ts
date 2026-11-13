import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { fakeHistory } from "./history.test-util";
import { createNavigation } from "./navigation";

// 書きかけがあるかと、確認への答えを決められる偽物。asked は、確認を出した回数。
function fakeGuard(dirty: boolean, answer: boolean) {
  const guard = {
    dirty,
    answer,
    asked: 0,
    anyDirty: () => guard.dirty,
    async confirmLeave() {
      guard.asked += 1;
      return guard.answer;
    },
  };
  return guard;
}

const settle = async () => {
  for (let i = 0; i < 5; i++) await Promise.resolve();
};
const tree = { name: "project", store: "work", project: "HH-P1", tab: "tree" } as const;
const treePath = "/s/work/p/HH-P1/tree";

describe("navigation with unsaved input", () => {
  it("moves at once and does not ask when nothing is dirty", () => {
    const { history } = fakeHistory("/");
    const guard = fakeGuard(false, false);
    const nav = createNavigation(history, guard);
    nav.go(tree);
    expect(get(nav.route)).toMatchObject({ name: "project" });
    expect(guard.asked).toBe(0);
  });

  it("asks first when something is dirty, and stays when canceled", async () => {
    const { history, stack } = fakeHistory("/");
    const guard = fakeGuard(true, false);
    const nav = createNavigation(history, guard);
    nav.go(tree);
    await settle();
    expect(guard.asked).toBe(1);
    expect(get(nav.route)).toEqual({ name: "home" });
    expect(stack).toEqual(["/"]);
  });

  it("moves once the input is discarded", async () => {
    const { history, stack } = fakeHistory("/");
    const nav = createNavigation(history, fakeGuard(true, true));
    nav.go(tree);
    await settle();
    expect(get(nav.route)).toMatchObject({ name: "project" });
    expect(stack).toEqual(["/", treePath]);
  });

  it("does not ask for replace (an internal address swap, not a move)", () => {
    const { history } = fakeHistory("/");
    const guard = fakeGuard(true, false);
    const nav = createNavigation(history, guard);
    nav.replace(tree);
    expect(get(nav.route)).toMatchObject({ name: "project" });
    expect(guard.asked).toBe(0);
  });

  it("puts the address back when the back button is canceled", async () => {
    const { history, stack, back } = fakeHistory("/");
    const guard = fakeGuard(false, false);
    const nav = createNavigation(history, guard);
    nav.start();
    nav.go(tree);
    guard.dirty = true;
    back();
    await settle();
    expect(guard.asked).toBe(1);
    expect(get(nav.route)).toMatchObject({ name: "project" });
    expect(history.path()).toBe(treePath);
    expect(stack).toEqual(["/", treePath]);
  });

  it("follows the back button once the input is discarded", async () => {
    const { history, back } = fakeHistory("/");
    const guard = fakeGuard(false, true);
    const nav = createNavigation(history, guard);
    nav.start();
    nav.go(tree);
    guard.dirty = true;
    back();
    await settle();
    expect(get(nav.route)).toEqual({ name: "home" });
  });
});
