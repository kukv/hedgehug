import { describe, expect, it } from "vitest";
import { Refused } from "../../model/errors";
import { get } from "../../reactive/store";
import { createToasts } from "../toasts/toasts";
import { createRulesEditor } from "./rulesEditor";

function setup() {
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  let changed = 0;
  const editor = createRulesEditor({
    toasts,
    onChanged: async () => {
      changed += 1;
    },
  });
  const texts = () => get(toasts.list).map((t) => ({ text: t.text, error: t.error === true }));
  return { editor, changed: () => changed, texts };
}

describe("createRulesEditor", () => {
  it("shows the refusal as an error toast and still reloads", async () => {
    const { editor, changed, texts } = setup();
    const ok = await editor.run(async () => {
      throw new Refused("初期ステータスのため外せません");
    }, "戻しました");
    expect(ok).toBe(false);
    expect(texts()).toEqual([{ text: "初期ステータスのため外せません", error: true }]);
    expect(changed()).toBe(1);
  });

  it("shows the ok toast after a successful operation and reloads", async () => {
    const { editor, changed, texts } = setup();
    const ok = await editor.run(async () => {}, "ワークスペースの設定に戻しました");
    expect(ok).toBe(true);
    expect(texts()).toEqual([{ text: "ワークスペースの設定に戻しました", error: false }]);
    expect(changed()).toBe(1);
  });

  it("shows no toast on success without an ok text", async () => {
    const { editor, changed, texts } = setup();
    expect(await editor.run(async () => {})).toBe(true);
    expect(texts()).toEqual([]);
    expect(changed()).toBe(1);
  });

  it("reloads and rethrows an error that is not a refusal", async () => {
    const { editor, changed, texts } = setup();
    await expect(
      editor.run(async () => {
        throw new Error("network");
      }),
    ).rejects.toThrow("network");
    expect(texts()).toEqual([]);
    expect(changed()).toBe(1);
  });
});
