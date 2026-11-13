import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createToasts } from "../toasts/toasts";
import { createCopyId } from "./copyId";

function setup(write: (text: string) => Promise<void>) {
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const copy = createCopyId({ write, toasts });
  const messages = () => get(toasts.list).map((t) => `${t.error ? "!" : ""}${t.text}`);
  return { copy, messages };
}

describe("createCopyId", () => {
  it("writes the ID and says so", async () => {
    const written: string[] = [];
    const { copy, messages } = setup(async (t) => void written.push(t));
    await copy("HH-3");
    expect(written).toEqual(["HH-3"]);
    expect(messages()).toEqual(["コピーしました: HH-3"]);
  });

  it("asks to select the ID by hand when the copy failed", async () => {
    const { copy, messages } = setup(async () => {
      throw new Error("denied");
    });
    await copy("HH-3");
    expect(messages()).toEqual(["!コピーできませんでした。HH-3 を選択してコピーしてください"]);
  });
});
