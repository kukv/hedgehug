import { describe, expect, it, vi } from "vitest";
import { get } from "../../reactive/store";
import { createToasts } from "../toasts/toasts";
import { fakeSyncPort } from "./fixtures.test-util";
import { createSync } from "./sync";

function setup(port = fakeSyncPort(), online = true) {
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const onSynced = vi.fn();
  const sync = createSync({ port, toasts, online: () => online, onSynced });
  const shown = () => get(toasts.list).map(({ text, error, warn }) => ({ text, error, warn }));
  return { port, sync, shown, onSynced };
}

describe("createSync", () => {
  it("loads the status", async () => {
    const at = new Date("2026-11-13T10:00:00Z");
    const { sync } = setup(
      fakeSyncPort({
        status: async () => ({
          auto: false,
          stores: [
            {
              storeId: "S1",
              name: "team",
              hidden: false,
              readOnly: false,
              error: "",
              unsent: 1,
              lastSuccess: at,
            },
          ],
        }),
      }),
    );
    expect(get(sync.status)).toBeNull();
    await sync.load();
    expect(get(sync.status)?.stores[0]?.lastSuccess).toEqual(at);
  });

  it("keeps the last status when loading fails (A2 の決定 29)", async () => {
    let fail = false;
    const { sync } = setup(
      fakeSyncPort({
        status: async () => {
          if (fail) throw new Error("down");
          return { auto: true, stores: [] };
        },
      }),
    );
    await sync.load();
    fail = true;
    await sync.load();
    expect(get(sync.status)).toEqual({ auto: true, stores: [] });
  });

  it("shows one toast for 「すべて同期」 and reloads (決定 S12・S13)", async () => {
    const { sync, shown, onSynced, port } = setup(
      fakeSyncPort({
        syncAll: async () => ({
          result: "failed",
          lines: ["成功 1 件・失敗 1 件", "team: アカウントが設定されていません"],
        }),
      }),
    );
    await sync.syncAll();
    expect(shown()).toEqual([
      {
        text: "成功 1 件・失敗 1 件\nteam: アカウントが設定されていません",
        error: true,
        warn: false,
      },
    ]);
    expect(onSynced).toHaveBeenCalledOnce();
    expect(port.calls).toEqual(["syncAll", "status"]);
  });

  it("shows a normal toast when there is nothing to sync", async () => {
    const { sync, shown } = setup(
      fakeSyncPort({
        syncAll: async () => ({
          result: "nothing",
          lines: ["同期する Store がありません(remote のある、表示中の Store が要ります)"],
        }),
      }),
    );
    await sync.syncAll();
    expect(shown()[0]).toMatchObject({ error: false });
  });

  it("shows the result of one store and the mirror warning after it", async () => {
    const { sync, shown } = setup(
      fakeSyncPort({
        syncStore: async () => ({
          result: "synced",
          message: "team を同期しました(送信 1 件・受信 2 件)",
          mirrorWarning: "ミラー 1 件に送れませんでした",
        }),
      }),
    );
    await sync.syncStore("S1");
    expect(shown()).toEqual([
      { text: "team を同期しました(送信 1 件・受信 2 件)", error: false, warn: false },
      { text: "ミラー 1 件に送れませんでした", error: false, warn: true },
    ]);
  });

  it("shows failed as an error and skipped as a normal toast", async () => {
    let result: "failed" | "skipped" = "failed";
    const { sync, shown } = setup(
      fakeSyncPort({ syncStore: async () => ({ result, message: result, mirrorWarning: "" }) }),
    );
    await sync.syncStore("S1");
    result = "skipped";
    await sync.syncStore("S1");
    expect(shown().map((t) => t.error)).toEqual([true, false]);
  });

  it("does not call the server while offline (決定 S15)", async () => {
    const { sync, shown, port, onSynced } = setup(fakeSyncPort(), false);
    await sync.syncAll();
    await sync.syncStore("S1");
    expect(port.calls).toEqual([]);
    expect(onSynced).not.toHaveBeenCalled();
    expect(shown()).toEqual([
      { text: "オフラインのため同期できません。つながったら同期します", error: true, warn: false },
      { text: "オフラインのため同期できません。つながったら同期します", error: true, warn: false },
    ]);
  });

  it("says there is nothing to sync when offline and no visible store has a remote", async () => {
    const { sync, shown, port } = setup(
      fakeSyncPort({ status: async () => ({ auto: true, stores: [] }) }),
      false,
    );
    await sync.load();
    await sync.syncAll();
    expect(port.calls).toEqual(["status"]);
    expect(shown()).toEqual([
      {
        text: "同期する Store がありません(remote のある、表示中の Store が要ります)",
        error: false,
        warn: false,
      },
    ]);
  });

  it("does not show a failed request (決定 S16)", async () => {
    const { sync, shown, onSynced } = setup(
      fakeSyncPort({
        syncAll: async () => {
          throw new Error("down");
        },
      }),
    );
    await sync.syncAll();
    expect(shown()).toEqual([]);
    expect(onSynced).not.toHaveBeenCalled();
  });
});
