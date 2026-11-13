import { describe, expect, it } from "vitest";
import type { Account, AccountRow } from "../../model/account";
import { Refused } from "../../model/errors";
import { get } from "../../reactive/store";
import { fakeAccounts, fakeToasts, row } from "./fixtures.test-util";
import { createAccountList } from "./list";

function setup(rows: AccountRow[]) {
  const f = fakeAccounts(rows);
  const { toasts, texts } = fakeToasts();
  const device: string[] = [];
  let refusal = "";
  const list = createAccountList({
    accounts: f.port,
    toasts,
    async reauthDevice(a: Account) {
      device.push(a.id);
      if (refusal) throw new Refused(refusal);
    },
  });
  const refuseDevice = (m: string) => {
    refusal = m;
  };
  return { f, texts, device, list, refuseDevice };
}

describe("createAccountList", () => {
  it("loads the rows when opened", async () => {
    const { list } = setup([row("A1"), row("A2", { status: "auth_error", reason: "x" })]);
    await list.open();
    expect(get(list.state).rows.map((r) => [r.id, r.status])).toEqual([
      ["A1", "ok"],
      ["A2", "auth_error"],
    ]);
  });

  it("reauths a gh account, tells it and reloads", async () => {
    const { f, list, texts } = setup([
      row("A1", { name: "work", login: "taro", status: "auth_error" }),
    ]);
    await list.open();
    await list.reauth("A1");
    expect(f.calls).toEqual(["reauth A1"]);
    expect(texts()).toEqual(["work: @taro として再認証しました"]);
    expect(get(list.state).rows[0]?.status).toBe("ok");
  });

  it("marks the row as asking while GitHub is asked, and does not ask twice", async () => {
    const { f, list } = setup([row("A1", { status: "auth_error" })]);
    await list.open();
    let release: () => void = () => undefined;
    const reauth = f.port.reauth;
    f.port.reauth = async (id) => {
      await new Promise<void>((r) => {
        release = r;
      });
      return reauth(id);
    };
    const first = list.reauth("A1");
    expect(get(list.state).asking).toEqual(["A1"]);
    await list.reauth("A1");
    release();
    await first;
    expect(get(list.state).asking).toEqual([]);
    expect(f.calls).toEqual(["reauth A1"]);
  });

  it("keeps a failed reauth in the row until the row is touched or the screen is opened again", async () => {
    const { f, list } = setup([row("A1", { status: "auth_error" })]);
    await list.open();
    f.refuse.reauth = "環境変数 GH_TOKEN が設定されていません";
    await list.reauth("A1");
    expect(get(list.state).errors).toEqual({ A1: "環境変数 GH_TOKEN が設定されていません" });
    list.touch("A1");
    expect(get(list.state).errors).toEqual({});
    await list.reauth("A1");
    await list.open();
    expect(get(list.state).errors).toEqual({});
  });

  it("opens the sign-in for a device flow account, and shows its refusal in the row", async () => {
    const { list, device, refuseDevice } = setup([
      row("A1", { method: "device", status: "auth_error" }),
    ]);
    await list.open();
    await list.reauth("A1");
    expect(device).toEqual(["A1"]);
    refuseDevice("GitHub に接続できません。接続を確かめて、もう一度お試しください");
    await list.reauth("A1");
    expect(get(list.state).errors.A1).toBe(
      "GitHub に接続できません。接続を確かめて、もう一度お試しください",
    );
  });

  it("asks with the impact before deleting, and deletes on confirm", async () => {
    const { f, list, texts } = setup([row("A1", { name: "work" })]);
    await list.open();
    f.impacts.delete = ["チーム の同期に使っているため、それらは同期できなくなります。"];
    await list.askDelete("A1");
    expect(get(list.state).deleting).toEqual({
      id: "A1",
      lines: [
        "「work」を削除します。",
        "チーム の同期に使っているため、それらは同期できなくなります。",
      ],
    });
    list.cancelDelete();
    expect(get(list.state).deleting).toBeNull();
    await list.askDelete("A1");
    await list.confirmDelete();
    expect(f.calls).toContain("remove A1");
    expect(texts()).toEqual(["アカウントを削除しました"]);
    expect(get(list.state).rows).toEqual([]);
    expect(get(list.state).deleting).toBeNull();
  });

  it("rechecks an app missing row, tells the cleared orgs and keeps the still orgs for the band", async () => {
    const { f, list, texts } = setup([
      row("A1", { name: "Taro", status: "app_missing", orgs: ["a", "b"] }),
    ]);
    await list.open();
    f.rechecks.push({ cleared: ["a"], still: ["b"] });
    await list.recheck(f.rows[0] as AccountRow);
    expect(f.calls).toEqual(["recheck A1"]);
    expect(texts()).toEqual(["Taro: a を読めるようになりました"]);
    expect(get(list.state).still).toEqual({ A1: ["b"] });
  });

  it("marks the row as asking during the recheck and does not ask twice", async () => {
    const { f, list } = setup([row("A1", { status: "app_missing", orgs: ["a"] })]);
    await list.open();
    let release: () => void = () => undefined;
    const recheck = f.port.recheck;
    f.port.recheck = async (id) => {
      await new Promise<void>((r) => {
        release = r;
      });
      return recheck(id);
    };
    const r = f.rows[0] as AccountRow;
    const first = list.recheck(r);
    expect(get(list.state).asking).toEqual(["A1"]);
    await list.recheck(r);
    release();
    await first;
    expect(get(list.state).asking).toEqual([]);
    expect(f.calls).toEqual(["recheck A1"]);
  });

  it("shows a failed recheck in the row, and clears it and the still orgs when the row is touched", async () => {
    const { f, list } = setup([row("A1", { status: "app_missing", orgs: ["a"] })]);
    await list.open();
    const r = f.rows[0] as AccountRow;
    f.rechecks.push({ cleared: [], still: ["a"] });
    await list.recheck(r);
    expect(get(list.state).still).toEqual({ A1: ["a"] });
    f.refuse.recheck = "GitHub に接続できません。接続を確かめて、もう一度お試しください";
    await list.recheck(r);
    expect(get(list.state).errors).toEqual({
      A1: "GitHub に接続できません。接続を確かめて、もう一度お試しください",
    });
    expect(get(list.state).still).toEqual({});
    f.refuse.recheck = "";
    delete f.refuse.recheck;
    f.rechecks.push({ cleared: [], still: ["a"] });
    await list.recheck(r);
    list.touch("A1");
    expect(get(list.state).still).toEqual({});
    await list.recheck(r);
    await list.open();
    expect(get(list.state).still).toEqual({});
  });

  it("reloads the row as an auth error when the recheck is refused because the token was rejected", async () => {
    const { f, list } = setup([row("A1", { status: "app_missing", orgs: ["a"] })]);
    await list.open();
    const message = "トークンが GitHub に拒否されました";
    f.port.recheck = async () => {
      const r = f.rows[0] as AccountRow;
      r.status = "auth_error";
      r.reason = message;
      r.orgs = [];
      throw new Refused(message);
    };
    await list.recheck(row("A1", { status: "app_missing", orgs: ["a"] }));
    expect(get(list.state).rows[0]?.status).toBe("auth_error");
    expect(get(list.state).errors).toEqual({ A1: message });
    expect(get(list.state).asking).toEqual([]);
  });

  it("keeps the red text and the still orgs of a row when another row is operated, until the row itself is operated", async () => {
    const { f, list } = setup([
      row("A1", { status: "app_missing", orgs: ["a"] }),
      row("A2", { status: "app_missing", orgs: ["b"] }),
    ]);
    await list.open();
    const [a1, a2] = f.rows as [AccountRow, AccountRow];
    f.rechecks.push({ cleared: [], still: ["a"] });
    await list.recheck(a1);
    f.rechecks.push({ cleared: [], still: ["b"] });
    await list.recheck(a2);
    expect(get(list.state).still).toEqual({ A1: ["a"], A2: ["b"] });
    f.refuse.recheck = "x";
    await list.recheck(a2);
    expect(get(list.state).still).toEqual({ A1: ["a"] });
    expect(get(list.state).errors).toEqual({ A2: "x" });
    delete f.refuse.recheck;
    await list.recheck(a1);
    expect(get(list.state).still).toEqual({ A1: [] });
    expect(get(list.state).errors).toEqual({ A2: "x" });
    await list.recheck(a2);
    expect(get(list.state).errors).toEqual({});
  });
  it("reloads the rows when the account of the recheck is gone", async () => {
    const { f, list, texts } = setup([row("A1", { status: "app_missing", orgs: ["a"] })]);
    await list.open();
    const gone = f.rows[0] as AccountRow;
    f.rows.splice(0, 1);
    await list.recheck(gone);
    expect(get(list.state).rows).toEqual([]);
    expect(get(list.state).errors).toEqual({});
    expect(get(list.state).asking).toEqual([]);
    expect(texts()).toEqual([]);
  });
});
