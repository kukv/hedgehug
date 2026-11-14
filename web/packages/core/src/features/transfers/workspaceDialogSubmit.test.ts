import { describe, expect, it } from "vitest";
import type { WorkspaceTransferPreview } from "../../model/transfer";
import { get } from "../../reactive/store";
import { flush } from "../items/fixtures.test-util";
import { setupWorkspace } from "./workspaceDialog.test-util";

const sent = (calls: string[]) => calls.filter((c) => c.startsWith("transferWorkspace"));

// opened は、移動先を選んで確認ダイアログを開いたところ。予告は defaultWorkspacePreview に patch を重ねたもの。
async function opened(key = "S3", patch: Partial<WorkspaceTransferPreview> = {}) {
  const s = await setupWorkspace();
  s.fake.defaultWorkspacePreview = { ...s.fake.defaultWorkspacePreview, ...patch };
  await s.transfer.load(s.workspace);
  s.transfer.choose(key);
  await s.transfer.open();
  return s;
}

describe("workspace transfer dialog: name check (B5c-16)", () => {
  it("does not send and shows「名前が一致しません」until the name matches", async () => {
    const { transfer, fake } = await opened();
    await transfer.submit("Mobile");
    expect(sent(fake.calls)).toEqual([]);
    expect(get(transfer.dialog)?.error).toBe("名前が一致しません");
    await transfer.submit("");
    expect(sent(fake.calls)).toEqual([]);
    transfer.clearError();
    expect(get(transfer.dialog)?.error).toBeNull();
  });

  it("sends when the trimmed input matches the name", async () => {
    const { transfer, fake } = await opened();
    await transfer.submit("  Web ");
    expect(sent(fake.calls)).toEqual(["transferWorkspace S1-W1 S3"]);
  });
});

describe("workspace transfer dialog: sending (B5c-16・B5c-18)", () => {
  it("toasts, calls afterMoved with the result and closes", async () => {
    const { transfer, fake, moved, toasts } = await opened("S3");
    await transfer.submit("Web");
    expect(toasts()).toEqual([{ text: "team へ移動しました", error: false }]);
    expect(moved).toEqual(["S1-W1 → team/HH"]);
    expect(get(transfer.dialog)).toBeNull();
    expect(sent(fake.calls)).toHaveLength(1);
  });

  it("asks afterMoved when「移動」is pressed, and passes the result only after the move", async () => {
    const { transfer, fake, moved, pressed } = await opened();
    const h = fake.holdNext();
    const sending = transfer.submit("Web");
    await flush();
    expect(pressed).toEqual(["S1-W1"]);
    expect(moved).toEqual([]);
    h.release();
    await sending;
    expect(moved).toHaveLength(1);
  });

  it("sends once while sending, and shows busy", async () => {
    const { transfer, fake } = await opened();
    const h = fake.holdNext();
    const sending = transfer.submit("Web");
    await flush();
    expect(get(transfer.dialog)?.busy).toBe(true);
    await transfer.submit("Web");
    h.release();
    await sending;
    expect(sent(fake.calls)).toHaveLength(1);
  });

  it("toasts a refusal and keeps the dialog open", async () => {
    const { transfer, fake, env, moved, toasts } = await opened();
    fake.refuseNext("team に送信できないため移動できません: 認証に失敗しました");
    await transfer.submit("Web");
    expect(toasts()).toEqual([
      { text: "team に送信できないため移動できません: 認証に失敗しました", error: true },
    ]);
    expect(get(transfer.dialog)?.busy).toBe(false);
    expect(moved).toEqual([]);
    expect(env.reloads).toBe(0);
  });

  it("toasts「移動できませんでした」on another failure and keeps the dialog open", async () => {
    const { transfer, fake, moved, toasts } = await opened();
    fake.failNext();
    await transfer.submit("Web");
    expect(moved).toEqual([]);
    expect(toasts()).toEqual([{ text: "移動できませんでした", error: true }]);
    expect(get(transfer.dialog)).not.toBeNull();
  });

  it("does not send to a Store with a remote while offline", async () => {
    const { transfer, fake, env, moved, pressed, toasts } = await opened("S3", {
      destinationHasRemote: true,
    });
    env.online = false;
    await transfer.submit("Web");
    expect(sent(fake.calls)).toEqual([]);
    expect(pressed).toEqual([]);
    expect(moved).toEqual([]);
    expect(toasts()).toEqual([{ text: "オフラインのため team へ移動できません", error: true }]);
    expect(get(transfer.dialog)).not.toBeNull();
  });

  it("sends to a Store without a remote while offline", async () => {
    const { transfer, fake, env } = await opened("S3", { destinationHasRemote: false });
    env.online = false;
    await transfer.submit("Web");
    expect(sent(fake.calls)).toHaveLength(1);
  });

  it("closes and reloads when the workspace is gone", async () => {
    const { transfer, fake, env, moved, toasts } = await opened();
    fake.missing.add("S1-W1");
    await transfer.submit("Web");
    expect(moved).toEqual([]);
    expect(get(transfer.dialog)).toBeNull();
    expect(toasts()).toEqual([{ text: "移動できませんでした", error: true }]);
    expect(env.reloads).toBe(1);
  });

  it("closes and reloads when the move could not be undone", async () => {
    const { transfer, fake, env, moved, toasts } = await opened();
    const text =
      "ワークスペース「Web」を team へ移しましたが、移動元から取り除けず、取り消しもできませんでした。どちらかを削除してください";
    fake.undoFailNext(text);
    await transfer.submit("Web");
    expect(get(transfer.dialog)).toBeNull();
    expect(toasts()).toEqual([{ text, error: true }]);
    expect(env.reloads).toBe(1);
    expect(moved).toEqual([]);
  });

  it("still toasts and calls afterMoved when cancelled while sending", async () => {
    const { transfer, fake, moved, toasts } = await opened();
    const h = fake.holdNext();
    const sending = transfer.submit("Web");
    await flush();
    transfer.cancel();
    expect(get(transfer.dialog)).toBeNull();
    h.release();
    await sending;
    expect(get(transfer.dialog)).toBeNull();
    expect(toasts()).toEqual([{ text: "team へ移動しました", error: false }]);
    expect(moved).toEqual(["S1-W1 → team/HH"]);
  });

  it("keeps the dialog closed when cancelled while a send fails", async () => {
    const { transfer, fake, moved, toasts } = await opened();
    const h = fake.holdNext();
    const sending = transfer.submit("Web");
    await flush();
    transfer.cancel();
    fake.failNext();
    h.release();
    await sending;
    expect(get(transfer.dialog)).toBeNull();
    expect(toasts()).toEqual([{ text: "移動できませんでした", error: true }]);
    expect(moved).toEqual([]);
  });
});
