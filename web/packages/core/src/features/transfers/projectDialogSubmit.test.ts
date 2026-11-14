import { describe, expect, it } from "vitest";
import type { ProjectTransferPreview } from "../../model/transfer";
import { get } from "../../reactive/store";
import { flush } from "../items/fixtures.test-util";
import { setupProject } from "./projectDialog.test-util";

const sent = (calls: string[]) => calls.filter((c) => c.startsWith("transferProject"));
const review = {
  status: "st9",
  name: "Review",
  count: 2,
  choices: [
    { id: "st1", name: "Todo" },
    { id: "st2", name: "Doing" },
  ],
  default: "st2",
};

// opened は、移動先を選んで確認ダイアログを開いたところ。予告は、置き換えの行 review に patch を重ねたもの。
async function opened(key = "S1/S1-W2", patch: Partial<ProjectTransferPreview> = {}) {
  const s = await setupProject();
  s.fake.defaultProjectPreview = {
    ...s.fake.defaultProjectPreview,
    replacements: [review],
    ...patch,
  };
  await s.transfer.load(s.project);
  s.transfer.choose(key);
  await s.transfer.open();
  return s;
}

describe("project transfer dialog: sending (02 **移動**、A5b-20)", () => {
  it("sends the destination and the status choices, then toasts and calls afterMoved", async () => {
    const { transfer, fake, moved, toasts } = await opened("S3/S3-W1");
    fake.projectResult = {
      toStore: "S3",
      projectId: "S1-P1",
      display: "TM-P4",
      destination: "team の Team",
    };
    transfer.setChoice("st9", "st1");
    await transfer.submit();
    expect(sent(fake.calls)).toEqual([
      `transferProject S1-P1 ${JSON.stringify({ toStore: "S3", toWorkspace: "S3-W1", statusChoices: { st9: "st1" } })}`,
    ]);
    expect(toasts()).toEqual([{ text: "team の Team へ移動しました", error: false }]);
    expect(moved).toEqual(["S1-P1 → S3/TM-P4"]);
    expect(get(transfer.dialog)).toBeNull();
  });

  it("sends once while sending, and shows busy", async () => {
    const { transfer, fake } = await opened();
    const h = fake.holdNext();
    const sending = transfer.submit();
    await flush();
    expect(get(transfer.dialog)?.busy).toBe(true);
    await transfer.submit();
    transfer.setChoice("st9", "st1");
    h.release();
    await sending;
    expect(sent(fake.calls)).toHaveLength(1);
    expect(fake.calls.at(-1)).toContain('"statusChoices":{"st9":"st2"}');
  });

  it("toasts a refusal and keeps the dialog open", async () => {
    const { transfer, fake, env, moved, toasts } = await opened();
    fake.refuseNext("Mobile には同じ名前のプロジェクトがあります");
    await transfer.submit();
    expect(toasts()).toEqual([
      { text: "Mobile には同じ名前のプロジェクトがあります", error: true },
    ]);
    expect(get(transfer.dialog)?.busy).toBe(false);
    expect(moved).toEqual([]);
    expect(env.reloads).toBe(0);
  });

  it("toasts「移動できませんでした」on another failure and keeps the dialog open", async () => {
    const { transfer, fake, toasts } = await opened();
    fake.failNext();
    await transfer.submit();
    expect(toasts()).toEqual([{ text: "移動できませんでした", error: true }]);
    expect(get(transfer.dialog)).not.toBeNull();
  });

  it("does not send to a Store with a remote while offline", async () => {
    const { transfer, fake, env, toasts } = await opened("S3/S3-W1", {
      crossStore: true,
      destinationHasRemote: true,
    });
    env.online = false;
    await transfer.submit();
    expect(sent(fake.calls)).toEqual([]);
    expect(toasts()).toEqual([{ text: "オフラインのため team へ移動できません", error: true }]);
    expect(get(transfer.dialog)).not.toBeNull();
  });

  it("sends within the same Store while offline", async () => {
    const { transfer, fake, env } = await opened("S1/S1-W2", { destinationHasRemote: true });
    env.online = false;
    await transfer.submit();
    expect(sent(fake.calls)).toHaveLength(1);
  });

  it("closes and reloads when the project is gone", async () => {
    const { transfer, fake, env, toasts } = await opened();
    fake.missing.add("S1-P1");
    await transfer.submit();
    expect(get(transfer.dialog)).toBeNull();
    expect(toasts()).toEqual([{ text: "移動できませんでした", error: true }]);
    expect(env.reloads).toBe(1);
  });

  it("closes and reloads when the move could not be undone", async () => {
    const { transfer, fake, env, moved, toasts } = await opened();
    const text =
      "HH-P1 を team の TM-P4 に移しましたが、移動元から取り除けず、取り消しもできませんでした。どちらかを削除してください";
    fake.undoFailNext(text);
    await transfer.submit();
    expect(get(transfer.dialog)).toBeNull();
    expect(toasts()).toEqual([{ text, error: true }]);
    expect(env.reloads).toBe(1);
    expect(moved).toEqual([]);
  });
});
