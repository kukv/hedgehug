import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createDrafts } from "./drafts";

describe("drafts", () => {
  it("holds only a non-empty input that differs; dirty is any difference", () => {
    const d = createDrafts();
    const box = { text: "", saved: "" };
    d.register(
      "description:item/I1",
      () => box.text !== box.saved,
      () => box.text !== box.saved && box.text !== "",
    );
    expect([d.anyHeld(), d.anyDirty()]).toEqual([false, false]);
    box.text = "書きかけ";
    expect([d.anyHeld(), d.anyDirty()]).toEqual([true, true]);
    box.saved = "書きかけ";
    expect([d.anyHeld(), d.anyDirty()]).toEqual([false, false]);
    // 保存してある説明を空にしようとしている: 確認は要るが、反映を待たせるほどの書きかけではない。
    box.saved = "古い説明";
    box.text = "";
    expect([d.anyHeld(), d.anyDirty()]).toEqual([false, true]);
  });

  it("holds as soon as it is dirty when no held is given, and unregistering removes it", () => {
    const d = createDrafts();
    const off = d.register("comment:item/I1", () => true);
    expect([d.anyHeld(), d.anyDirty()]).toEqual([true, true]);
    off();
    expect([d.anyHeld(), d.anyDirty()]).toEqual([false, false]);
  });

  it("keeps a registration made again under the same key when the old one is removed", () => {
    const d = createDrafts();
    const old = d.register("description:item/I1", () => true);
    d.register("description:item/I1", () => true);
    old();
    expect(d.anyDirty()).toBe(true);
  });

  it("answers true at once when nothing is dirty", async () => {
    const d = createDrafts();
    d.register("description:item/I1", () => false);
    expect(await d.confirmLeave()).toBe(true);
    expect(get(d.prompt)).toBeNull();
  });

  it("asks once, with a line per kind, and answers with the button", async () => {
    const d = createDrafts();
    d.register("comment:item/I1", () => true);
    d.register("description:item/I1", () => true);
    d.register("comment:item/I1/2", () => true);
    const first = d.confirmLeave();
    const second = d.confirmLeave();
    expect(get(d.prompt)).toEqual({
      title: "入力を破棄",
      lines: ["保存していないコメントを破棄します。", "保存していない説明の変更を破棄します。"],
      ok: "破棄",
    });
    d.answer(true);
    expect([await first, await second]).toEqual([true, true]);
    expect(get(d.prompt)).toBeNull();
    const third = d.confirmLeave();
    d.answer(false);
    expect(await third).toBe(false);
  });

  it("asks only about the given key", async () => {
    const d = createDrafts();
    d.register("comment:item/I1", () => true);
    d.register("description:item/I1", () => false);
    expect(await d.confirmLeave("description:item/I1")).toBe(true);
    const asked = d.confirmLeave("comment:item/I1");
    expect(get(d.prompt)?.lines).toEqual(["保存していないコメントを破棄します。"]);
    d.answer(false);
    expect(await asked).toBe(false);
  });
});
