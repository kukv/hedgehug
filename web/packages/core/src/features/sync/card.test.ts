import { describe, expect, it } from "vitest";
import type { SyncStatus, SyncStoreStatus } from "../../model/sync";
import { hasSyncError, isSyncAllKey, offlineBand, syncAllHint, syncCard } from "./card";

const now = new Date(2026, 10, 13, 12, 0, 0);
const hoursAgo = (h: number) => new Date(now.getTime() - h * 3_600_000);

function store(over: Partial<SyncStoreStatus>): SyncStoreStatus {
  return {
    storeId: "S1",
    name: "team-a",
    hidden: false,
    readOnly: false,
    error: "",
    unsent: 0,
    lastSuccess: hoursAgo(1),
    ...over,
  };
}
const status = (...stores: SyncStoreStatus[]): SyncStatus => ({ auto: true, stores });

describe("syncCard (06「同期の状態の表示」)", () => {
  it("is absent without a visible store that has a remote", () => {
    expect(syncCard(null, now)).toBeNull();
    expect(syncCard(status(), now)).toBeNull();
    expect(syncCard(status(store({ hidden: true })), now)).toBeNull();
  });

  it("shows すべて最新 when nothing is unsent", () => {
    const c = syncCard(status(store({}), store({ storeId: "S2", name: "solo" })), now);
    expect(c).toMatchObject({
      summary: "すべて最新",
      error: false,
      last: "最終同期 1 時間前 · Store 2 件",
      stale: null,
    });
  });

  it("sums unsent saves", () => {
    const c = syncCard(status(store({ unsent: 2 }), store({ storeId: "S2", unsent: 3 })), now);
    expect(c?.summary).toBe("未送信 5");
  });

  it("counts sync errors before unsent, and does not count hidden stores", () => {
    const c = syncCard(
      status(
        store({ error: "認証に失敗しました", unsent: 2 }),
        store({ storeId: "S2", hidden: true, error: "認証に失敗しました" }),
      ),
      now,
    );
    expect(c).toMatchObject({
      summary: "1 件の Store で同期エラー",
      error: true,
      last: "最終同期 1 時間前 · Store 1 件",
    });
  });

  it("uses the newest success and なし when no store has synced", () => {
    expect(
      syncCard(
        status(
          store({ lastSuccess: hoursAgo(5) }),
          store({ storeId: "S2", lastSuccess: hoursAgo(2) }),
        ),
        now,
      )?.last,
    ).toBe("最終同期 2 時間前 · Store 2 件");
    expect(syncCard(status(store({ lastSuccess: null })), now)?.last).toBe(
      "最終同期 なし · Store 1 件",
    );
  });

  it("warns about stores not synced for 24 hours, even with an error (06 の例)", () => {
    const c = syncCard(
      status(
        store({ error: "認証に失敗しました", lastSuccess: hoursAgo(48) }),
        store({ storeId: "S2", lastSuccess: null }),
      ),
      now,
    );
    expect(c?.summary).toBe("1 件の Store で同期エラー");
    expect(c?.stale).toBe("1 件の Store が 24 時間以上同期されていません");
  });

  it("lists the rows of 「Store ごとの同期状態」", () => {
    const c = syncCard(
      status(
        store({ error: "認証に失敗しました", lastSuccess: hoursAgo(2) }),
        store({ storeId: "S2", name: "solo", unsent: 3, lastSuccess: hoursAgo(30) }),
        store({ storeId: "S3", name: "docs", lastSuccess: null }),
        store({ storeId: "S4", name: "old", hidden: true }),
      ),
      now,
    );
    expect(c?.rows).toEqual([
      {
        storeId: "S1",
        name: "team-a",
        summary: "同期エラー: 認証に失敗しました",
        time: "2 時間前",
        stale: false,
      },
      { storeId: "S2", name: "solo", summary: "未送信 3", time: "1 日前", stale: true },
      { storeId: "S3", name: "docs", summary: "最新", time: "未同期", stale: false },
    ]);
  });
});

describe("hasSyncError (860px 以下の点)", () => {
  it("counts visible stores only, read-only included", () => {
    expect(hasSyncError(null)).toBe(false);
    expect(hasSyncError(status(store({ readOnly: true, error: "認証に失敗しました" })))).toBe(true);
    expect(hasSyncError(status(store({ hidden: true, error: "認証に失敗しました" })))).toBe(false);
  });
});

describe("offlineBand (06「オフライン」)", () => {
  it("changes with the automatic sync setting", () => {
    expect(offlineBand(true)).toBe(
      "オフラインです。変更はこのマシンにコミットし、つながったら push します。",
    );
    expect(offlineBand(false)).toBe(
      "オフラインです。変更はこのマシンにコミットします。つながったら「すべて同期」で push してください。",
    );
  });
});

describe("isSyncAllKey (Shift+Ctrl+S、Mac は Shift+Cmd+S)", () => {
  const key = (over: Partial<Parameters<typeof isSyncAllKey>[0]>) => ({
    key: "S",
    shiftKey: true,
    ctrlKey: false,
    metaKey: false,
    altKey: false,
    ...over,
  });
  it("accepts Ctrl or Cmd with Shift", () => {
    expect(isSyncAllKey(key({ ctrlKey: true }))).toBe(true);
    expect(isSyncAllKey(key({ metaKey: true, key: "s" }))).toBe(true);
  });
  it("rejects other combinations", () => {
    expect(isSyncAllKey(key({ ctrlKey: true, shiftKey: false }))).toBe(false);
    expect(isSyncAllKey(key({}))).toBe(false);
    expect(isSyncAllKey(key({ ctrlKey: true, altKey: true }))).toBe(false);
    expect(isSyncAllKey(key({ ctrlKey: true, key: "a" }))).toBe(false);
  });
  it("names the shortcut for the platform", () => {
    expect(syncAllHint(true)).toBe("Shift+Cmd+S");
    expect(syncAllHint(false)).toBe("Shift+Ctrl+S");
  });
});
