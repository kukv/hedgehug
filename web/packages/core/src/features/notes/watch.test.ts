import { describe, expect, it, vi } from "vitest";
import { Refused } from "../../model/errors";
import type { Item } from "../../model/item";
import { get } from "../../reactive/store";
import { item as makeItem } from "../items/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createWatch, watchView } from "./watch";

const watched = (watchers: string[]): Item => ({
  ...makeItem("S1", "S1-P1", 2, "Task", "S1-I1", "画面"),
  watchers,
});

describe("watchView", () => {
  it("shows which state it is in, without caring about case", () => {
    expect(watchView(watched(["taro"]), "taro", false)).toEqual({
      watching: true,
      label: "ウォッチ中",
      disabled: false,
      note: undefined,
    });
    expect(watchView(watched(["Taro"]), "taro", false).watching).toBe(true);
    expect(watchView(watched(["hanako"]), "taro", false)).toMatchObject({
      watching: false,
      label: "ウォッチ",
    });
  });

  it("is disabled in a read-only Store, with the note, and when me is not decided", () => {
    expect(watchView(watched([]), "taro", true)).toMatchObject({
      disabled: true,
      note: "閲覧のみの Store です",
    });
    expect(watchView(watched(["taro"]), "", false)).toMatchObject({
      watching: false,
      disabled: true,
    });
  });
});

function setup() {
  const notes = { setWatch: vi.fn(async (_s: string, _i: string, _on: boolean) => watched([])) };
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const changed = vi.fn();
  const watch = createWatch({ notes, toasts, onChanged: changed });
  return { watch, notes, toasts, changed };
}

describe("createWatch", () => {
  it("starts watching when not watching, then reloads, with no toast", async () => {
    const { watch, notes, toasts, changed } = setup();
    await watch.toggle(watched(["hanako"]), "taro");
    expect(notes.setWatch).toHaveBeenCalledWith("S1", "S1-I2", true);
    expect(changed).toHaveBeenCalledOnce();
    expect(get(toasts.list)).toEqual([]);
  });

  it("stops watching when watching, whatever the case of the login", async () => {
    const { watch, notes } = setup();
    await watch.toggle(watched(["Taro"]), "taro");
    expect(notes.setWatch).toHaveBeenCalledWith("S1", "S1-I2", false);
  });

  it("ignores a second press while sending, and when me is not decided", async () => {
    const { watch, notes } = setup();
    const first = watch.toggle(watched([]), "taro");
    expect(get(watch.busy)).toBe(true);
    await watch.toggle(watched([]), "taro");
    await first;
    expect(notes.setWatch).toHaveBeenCalledOnce();
    expect(get(watch.busy)).toBe(false);
    await watch.toggle(watched([]), "");
    expect(notes.setWatch).toHaveBeenCalledOnce();
  });

  it("shows a refusal in a toast and does not reload", async () => {
    const { watch, notes, toasts, changed } = setup();
    notes.setWatch.mockRejectedValueOnce(new Refused("断り"));
    await watch.toggle(watched([]), "taro");
    expect(get(toasts.list)[0]).toMatchObject({ text: "断り", error: true });
    expect(changed).not.toHaveBeenCalled();
    expect(get(watch.busy)).toBe(false);
  });
});
