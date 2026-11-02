import { CompletionContext } from "@codemirror/autocomplete";
import { EditorState } from "@codemirror/state";
import { describe, expect, it, vi } from "vitest";
import type { MentionDeps } from "./mentions";
import { mentionSource, triggerOf } from "./mentions";
import { taskView } from "./testkit";

const noDeps: MentionDeps = { people: vi.fn(async () => []), search: vi.fn(async () => []) };

const at = (doc: string, deps: MentionDeps = noDeps) =>
  mentionSource(
    "shared",
    deps,
  )(new CompletionContext(EditorState.create({ doc }), doc.length, false));

describe("triggerOf", () => {
  it("reads @ and # that start a word", () => {
    expect(triggerOf("@")).toEqual({ sigil: "@", typed: "" });
    expect(triggerOf("了解です。@suz")).toEqual({ sigil: "@", typed: "suz" });
    expect(triggerOf("see #REL-4")).toEqual({ sigil: "#", typed: "REL-4" });
    expect(triggerOf("(#REL")).toEqual({ sigil: "#", typed: "REL" });
  });

  it("ignores a sigil inside a word and text that has ended the word", () => {
    expect(triggerOf("me@example")).toBeNull();
    expect(triggerOf("a#b")).toBeNull();
    expect(triggerOf("@suzuki done")).toBeNull();
    expect(triggerOf("plain")).toBeNull();
  });
});

describe("mentionSource", () => {
  it("offers the people whose names match what follows @", async () => {
    const people = vi.fn(async () => ["suzuki", "suzumura", "tanaka"]);
    const r = await at("確認は @suz", { ...noDeps, people });
    expect(people).toHaveBeenCalledWith("shared");
    expect(r?.from).toBe(5);
    expect(r?.options.map((o) => [o.label, o.apply])).toEqual([
      ["suzuki", "suzuki "],
      ["suzumura", "suzumura "],
    ]);
  });

  it("offers the tasks the search finds for #, with the title as the detail", async () => {
    const search = vi.fn(async () => [taskView({ id: "REL-4", title: "移行ガイドを書く" })]);
    const r = await at("#REL", { ...noDeps, search });
    expect(search).toHaveBeenCalledWith("REL", "shared");
    expect(r?.options[0]).toMatchObject({
      label: "REL-4",
      detail: "移行ガイドを書く",
      apply: "REL-4 ",
    });
  });

  it("asks nothing for a bare # and offers nothing without a trigger", async () => {
    const search = vi.fn(async () => []);
    expect(await at("#", { ...noDeps, search })).toBeNull();
    expect(search).not.toHaveBeenCalled();
    expect(await at("hello")).toBeNull();
  });
});
