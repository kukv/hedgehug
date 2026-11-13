import { describe, expect, it } from "vitest";
import { NotFound } from "../../model/errors";
import { get } from "../../reactive/store";
import { createToasts } from "../toasts/toasts";
import { createDescriptionEditor, descriptionConflictMessage } from "./description";
import { fakeNotesPort, keyOf } from "./fixtures.test-util";

const target = { kind: "item", storeId: "S1", id: "I1" } as const;
const first = "## 手順\n@taro を見る";

function setup() {
  const notes = fakeNotesPort({ [keyOf(target)]: first });
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const editor = createDescriptionEditor({
    port: notes,
    toasts,
    target: () => target,
    members: async () => [{ login: "taro", name: "太郎" }],
  });
  const shown = () => get(toasts.list).map((t) => t.text);
  return { notes, toasts, editor, shown };
}

describe("description editor", () => {
  it("shows nothing until it has loaded, then the text and its Markdown with member mentions", async () => {
    const { editor } = setup();
    expect(get(editor.state)).toMatchObject({ loaded: false, text: "", blocks: [] });
    await editor.load();
    const s = get(editor.state);
    expect(s.loaded).toBe(true);
    expect(s.text).toBe(first);
    expect(s.blocks).toEqual([
      { kind: "heading", level: 2, inlines: [{ kind: "text", text: "手順" }] },
      {
        kind: "paragraph",
        inlines: [
          { kind: "mention", login: "taro" },
          { kind: "text", text: " を見る" },
        ],
      },
    ]);
  });

  it("is dirty when the input differs from where editing began, and held only when it is not empty", async () => {
    const { editor } = setup();
    editor.start(); // 読む前は始まらない
    expect(get(editor.state).editing).toBe(false);
    await editor.load();
    editor.start();
    expect([editor.dirty(), editor.held()]).toEqual([false, false]);
    editor.setInput("変えた");
    expect([editor.dirty(), editor.held()]).toEqual([true, true]);
    editor.setInput("");
    expect([editor.dirty(), editor.held()]).toEqual([true, false]);
    editor.setInput(first);
    expect([editor.dirty(), editor.held()]).toEqual([false, false]);
  });

  it("saves with the text where editing began as the common ancestor", async () => {
    const { editor, notes, shown } = setup();
    await editor.load();
    editor.start();
    editor.setInput("## 手順\n書き換えた");
    await editor.save();
    expect(notes.describeCalls).toEqual([{ base: first, text: "## 手順\n書き換えた" }]);
    expect(get(editor.state)).toMatchObject({ editing: false, text: "## 手順\n書き換えた" });
    expect(shown()).toEqual([]);
  });

  it("saves nothing and closes when the input is the same as before", async () => {
    const { editor, notes } = setup();
    await editor.load();
    editor.start();
    await editor.save();
    expect(notes.describeCalls).toEqual([]);
    expect(get(editor.state).editing).toBe(false);
  });

  it("shows the merged text and the toast when both versions were kept", async () => {
    const { editor, notes, shown } = setup();
    await editor.load();
    editor.start();
    editor.setInput("自分の版");
    notes.conflictNext("-----\n自分の版\n-----\n他の人の版\n-----");
    await editor.save();
    expect(get(editor.state).text).toBe("-----\n自分の版\n-----\n他の人の版\n-----");
    expect(shown()).toEqual([descriptionConflictMessage]);
    expect(descriptionConflictMessage).toBe(
      "説明の同じ箇所が同時に編集されたため、両方を残しました",
    );
  });

  it("keeps the input and the editor open when the save is refused", async () => {
    const { editor, notes, toasts } = setup();
    await editor.load();
    editor.start();
    editor.setInput("変えた");
    notes.refuseNext("閲覧のみの Store です");
    await editor.save();
    expect(get(toasts.list)[0]).toMatchObject({ text: "閲覧のみの Store です", error: true });
    expect(get(editor.state)).toMatchObject({ editing: true, input: "変えた", saving: false });
  });

  it("does not rewrite the input when the saved text changes meanwhile (reflection)", async () => {
    const { editor, notes } = setup();
    await editor.load();
    editor.start();
    editor.setInput("入力中");
    notes.texts.set(keyOf(target), "他の人の説明");
    await editor.load();
    expect(get(editor.state)).toMatchObject({ text: "他の人の説明", input: "入力中", base: first });
    // 保存では、編集を始めたときの説明が祖先のまま。
    await editor.save();
    expect(notes.describeCalls[0]?.base).toBe(first);
  });

  it("does not make an untouched input dirty when the saved text changes", async () => {
    const { editor, notes } = setup();
    await editor.load();
    editor.start();
    notes.texts.set(keyOf(target), "他の人の説明");
    await editor.load();
    expect([editor.dirty(), editor.held()]).toEqual([false, false]);
  });

  it("throws the input away on cancel without saving", async () => {
    const { editor, notes } = setup();
    await editor.load();
    editor.start();
    editor.setInput("捨てる");
    editor.cancel();
    expect(get(editor.state)).toMatchObject({ editing: false, input: "", base: "" });
    expect(editor.dirty()).toBe(false);
    expect(notes.describeCalls).toEqual([]);
  });

  it("ignores an item that is gone (the panel closes through the reflection)", async () => {
    const { editor, notes } = setup();
    notes.description = async () => {
      throw new NotFound("gone");
    };
    await editor.load();
    expect(get(editor.state).loaded).toBe(false);
  });

  it("forgets the previous target's text and input on reset, and reads the new one", async () => {
    const other = { kind: "item", storeId: "S1", id: "I2" } as const;
    const notes = fakeNotesPort({ [keyOf(target)]: first, [keyOf(other)]: "別の説明" });
    let now: typeof target | typeof other = target;
    const toasts = createToasts({ set: () => 0, clear: () => {} });
    const editor = createDescriptionEditor({
      port: notes,
      toasts,
      target: () => now,
      members: async () => [],
    });
    await editor.load();
    editor.start();
    editor.setInput("前の入力");
    now = other;
    editor.reset();
    expect(get(editor.state)).toMatchObject({
      loaded: false,
      text: "",
      editing: false,
      input: "",
      base: "",
    });
    expect([editor.dirty(), editor.held()]).toEqual([false, false]);
    await editor.load();
    expect(get(editor.state)).toMatchObject({
      loaded: true,
      text: "別の説明",
      editing: false,
      input: "",
    });
  });
});
