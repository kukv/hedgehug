import { describe, expect, it } from "vitest";
import { ApiError } from "./api/client";
import { errorMessage } from "./messages";

const err = (
  type: string,
  args: Record<string, unknown> = {},
  detail = "english",
  body?: unknown,
) => new ApiError({ status: 400, type, detail, args, body });

describe("errorMessage", () => {
  it("puts the args into the words", () => {
    expect(errorMessage(err("task.not_found", { input: "REL-9" }))).toBe(
      "REL-9 というタスクは見つかりません",
    );
    expect(errorMessage(err("comment.not_author", { author: "alice" }))).toBe(
      "alice が書いたコメントは変更できません",
    );
    expect(errorMessage(err("assign.bad_value", { key: "points", value: "x" }, ""))).toBe(
      "points に「x」は指定できません",
    );
    expect(errorMessage(err("store.not_found", { name: "foo", candidates: ["a", "b"] }))).toBe(
      "foo という Store はありません（登録済み: a、b）",
    );
  });

  it("words restore_at.not_found for a restore and for reading a point in time", () => {
    expect(errorMessage(err("restore_at.not_found", { at: "abc123" }))).toBe(
      "abc123 の時点にはこのタスクがありませんでした",
    );
    expect(errorMessage(err("restore_at.not_found"))).toBe(
      "その時点にはこのタスクがありませんでした",
    );
  });

  it("words task.moved with and without the store and the new ID", () => {
    expect(errorMessage(err("task.moved", { input: "REL-9", store: "work", id: "REL-10" }))).toBe(
      "REL-9 は work へ移りました（今の ID は REL-10）",
    );
    expect(errorMessage(err("task.moved", { input: "REL-9", store: "work" }))).toBe(
      "REL-9 は work へ移りました",
    );
    expect(errorMessage(err("task.moved", { input: "REL-9" }))).toBe(
      "REL-9 は別の Store へ移りました",
    );
  });

  it("words restore.forward with the point in time", () => {
    expect(errorMessage(err("restore.forward", { at: "abc123" }))).toBe(
      "abc123 の時点では、このタスクは別の Store へ移っていました",
    );
  });

  it("words sync.force_push", () => {
    expect(errorMessage(err("sync.force_push"))).toBe(
      "リモートの履歴が手元と食い違っています。確かめてから、hh doctor --fix で clone し直してください",
    );
  });

  it("tells the two auth.token apart", () => {
    expect(errorMessage(err("auth.token", { account: "work" }))).toContain("work");
    expect(errorMessage(err("auth.token"))).toBe(
      "この画面は hh ui が開いた URL からだけ使えます。端末で hh ui を実行してください",
    );
  });

  it("counts the targets of purge.unconfirmed", () => {
    const targets = [{ ulid: "a" }, { ulid: "b" }];
    expect(errorMessage(err("purge.unconfirmed", { targets }))).toContain("2 件");
  });

  it("lists what a schema violation broke", () => {
    const m = errorMessage(
      err("schema.violation", {
        violations: [
          { code: "placement", field: "parent", message: "a epic cannot be placed under task" },
        ],
      }),
    );
    expect(m).toContain("この親には置けません（parent）");
  });

  it("joins the details of request.invalid", () => {
    const body = { errors: [{ message: "expected string" }, { message: "too short" }] };
    expect(errorMessage(err("request.invalid", {}, "validation failed", body))).toBe(
      "入力が正しくありません: expected string / too short",
    );
  });

  it("words the errors of a position given to an add", () => {
    expect(errorMessage(err("add.position", {}, "ME-3 is not a sibling"))).toBe(
      "この位置には作れません: ME-3 is not a sibling",
    );
    expect(errorMessage(err("position.both"))).toBe(
      "前に置くか後ろに置くか、どちらか一方を指定してください",
    );
  });

  it("shows the type and the detail of an unknown type", () => {
    expect(errorMessage(err("weird.thing", {}, "boom"))).toBe(
      "操作に失敗しました（weird.thing）: boom",
    );
  });

  it("words an error that is not an ApiError", () => {
    expect(errorMessage(new TypeError("Failed to fetch"))).toBe(
      "操作に失敗しました: Failed to fetch",
    );
    expect(errorMessage("x")).toBe("操作に失敗しました: x");
  });

  it("says in Japanese that the server cannot be reached", () => {
    expect(
      errorMessage(new ApiError({ status: 0, type: "network", detail: "Failed to fetch" })),
    ).toBe("サーバーに接続できません");
  });
});
