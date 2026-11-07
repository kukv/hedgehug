import { describe, expect, it } from "vitest";
import type { SyncStatus } from "../../core/connection";
import { summarizeSync, syncMessage } from "./sync-summary";

const st = (over: Partial<SyncStatus> & { store: string }): SyncStatus => ({
  remotes: ["origin"],
  interval: 120,
  push_interval: 30,
  ...over,
});

describe("summarizeSync", () => {
  it("remote のある Store がなければ同期済みで noRemote", () => {
    const s = summarizeSync([st({ store: "me", remotes: [] })], true);
    expect(s.kind).toBe("synced");
    expect(s.noRemote).toBe(true);
  });

  it("未 push がなくエラーもなければ同期済み。最終同期は一番新しいもの", () => {
    const s = summarizeSync(
      [
        st({
          store: "a",
          unpushed: 0,
          last_success: "2026-11-02T09:00:00Z",
          next_fetch: "2026-11-02T09:05:00Z",
        }),
        st({
          store: "b",
          unpushed: 0,
          last_success: "2026-11-02T09:02:00Z",
          next_fetch: "2026-11-02T09:04:00Z",
        }),
      ],
      true,
    );
    expect(s).toMatchObject({
      kind: "synced",
      noRemote: false,
      unpushed: 0,
      lastSuccess: "2026-11-02T09:02:00Z",
      nextFetch: "2026-11-02T09:04:00Z",
    });
  });

  it("未 push があればpush 待ち。合計と一番古い最初の変更を出す", () => {
    const s = summarizeSync(
      [
        st({
          store: "a",
          unpushed: 2,
          pending_since: "2026-11-02T09:01:00Z",
          push_at: "2026-11-02T09:03:00Z",
        }),
        st({
          store: "b",
          unpushed: 1,
          pending_since: "2026-11-02T09:00:00Z",
          push_at: "2026-11-02T09:02:00Z",
        }),
      ],
      true,
    );
    expect(s).toMatchObject({
      kind: "pending",
      unpushed: 3,
      pendingSince: "2026-11-02T09:00:00Z",
      pushAt: "2026-11-02T09:02:00Z",
    });
  });

  it("last_error があれば push できません。Store 名と文言を持つ", () => {
    const s = summarizeSync(
      [
        st({ store: "a", unpushed: 1, last_error: "push rejected" }),
        st({ store: "b", unpushed: 0 }),
      ],
      true,
    );
    expect(s.kind).toBe("error");
    expect(s.errors).toEqual([{ store: "a", message: "push rejected" }]);
  });

  it("読めない Store の error も push できませんにする", () => {
    const s = summarizeSync([st({ store: "a", error: "cannot read store" })], true);
    expect(s.kind).toBe("error");
    expect(s.errors[0]?.message).toBe("cannot read store");
  });

  it("オフラインはエラーや未 push より優先する", () => {
    const s = summarizeSync([st({ store: "a", unpushed: 5, last_error: "x" })], false);
    expect(s).toMatchObject({ kind: "offline", unpushed: 5 });
  });

  it("remote がなければオフラインでも同期済み（remote なし）", () => {
    const s = summarizeSync([st({ store: "me", remotes: [] })], false);
    expect(s).toMatchObject({ kind: "synced", noRemote: true });
  });

  it("remote のない Store の未 push は数えない", () => {
    const s = summarizeSync(
      [st({ store: "me", remotes: [], unpushed: 4 }), st({ store: "a", unpushed: 0 })],
      true,
    );
    expect(s.unpushed).toBe(0);
  });
});

describe("syncMessage", () => {
  const rep = (over: Record<string, unknown>) => ({
    store: "a",
    merges: 0,
    pushed: [],
    overwritten: [],
    repaired: [],
    invalid: [],
    notices: [],
    warnings: [],
    ...over,
  });

  it("失敗は Store 名と文言を出す", () => {
    expect(syncMessage([rep({ error: "auth failed" }) as never])).toBe(
      "同期に失敗しました: a: auth failed",
    );
  });

  it("remote がなければそう言う", () => {
    expect(syncMessage([rep({ no_remote: true }) as never])).toBe("同期する remote がありません");
    expect(syncMessage([])).toBe("同期する remote がありません");
    expect(syncMessage(null)).toBe("同期する remote がありません");
  });

  it("成功はマージの数を添える", () => {
    expect(syncMessage([rep({}) as never])).toBe("同期しました");
    expect(
      syncMessage([rep({ merges: 2 }) as never, rep({ store: "b", merges: 1 }) as never]),
    ).toBe("同期しました · マージ 3 件");
  });
});
