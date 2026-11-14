import { describe, expect, it } from "vitest";
import type { BulkChangeResult } from "../../model/bulkChange";
import { bulkChangeToast, offBacklogText, reasonList } from "./bulkChangeResult";
import type { SelectedRow } from "./rowSelection";

const rows: SelectedRow[] = [1, 2, 3, 4, 5].map((n) => ({
  storeId: "s",
  itemId: `i${n}`,
  display: `HH-${n}`,
  readOnly: false,
}));
const kept = (n: number, reason: string) => ({ store: "s", item: `i${n}`, reason });
const base: BulkChangeResult = {
  changed: 2,
  kept: [],
  failed: [],
  offBacklog: { done: 0, canceled: 0 },
  valueText: "",
};
const some: BulkChangeResult = { ...base, kept: [kept(5, "削除済み")] };

describe("reason list", () => {
  const r: BulkChangeResult = {
    ...base,
    kept: [
      kept(1, "閲覧のみ"),
      kept(2, "閲覧のみ"),
      kept(3, "閲覧のみ"),
      kept(4, "閲覧のみ"),
      kept(5, "削除済み"),
    ],
  };
  it("groups by reason, 3 ids then more", () => {
    expect(reasonList(r, rows)).toBe("HH-1・HH-2・HH-3 ほか 1 件(閲覧のみ)、HH-5(削除済み)");
  });
  it("adds failed stores", () => {
    const f = { ...r, failed: [{ store: "t", name: "team", count: 2 }] };
    expect(reasonList(f, rows)).toBe(
      "HH-1・HH-2・HH-3 ほか 1 件(閲覧のみ)、HH-5(削除済み)、team の 2 件(保存できませんでした)",
    );
  });
  it("falls back to the internal id", () => {
    const x = { ...base, kept: [{ store: "s", item: "zzz", reason: "削除済み" }] };
    expect(reasonList(x, rows)).toBe("zzz(削除済み)");
  });
  it("matches the store too", () => {
    const x = { ...base, kept: [{ store: "other", item: "i1", reason: "削除済み" }] };
    expect(reasonList(x, rows)).toBe("i1(削除済み)");
  });
});

describe("toast", () => {
  it("all changed", () => {
    expect(bulkChangeToast(base, rows, "")).toEqual({ text: "2 件を変えました", error: false });
  });
  it("partly changed", () => {
    expect(bulkChangeToast(some, rows, "")).toEqual({
      text: "2 件を変えました。1 件はそのままです: HH-5(削除済み)",
      error: true,
    });
  });
  it("counts failed stores as kept", () => {
    const r = { ...base, failed: [{ store: "t", name: "team", count: 2 }] };
    expect(bulkChangeToast(r, rows, "").text).toBe(
      "2 件を変えました。2 件はそのままです: team の 2 件(保存できませんでした)",
    );
  });
  it("none changed", () => {
    expect(bulkChangeToast({ ...some, changed: 0 }, rows, "")).toEqual({
      text: "変更できたものはありません: HH-5(削除済み)",
      error: true,
    });
    expect(bulkChangeToast({ ...base, changed: 0 }, rows, "")).toEqual({
      text: "変わるものはありません",
      error: true,
    });
  });
  it("suffix only when something changed", () => {
    expect(bulkChangeToast(base, rows, "(元に戻せます)").text).toBe(
      "2 件を変えました(元に戻せます)",
    );
    expect(bulkChangeToast({ ...base, changed: 0 }, rows, "(x)").text).toBe(
      "変わるものはありません",
    );
  });
  it("off backlog text is not added unless passed as suffix", () => {
    const r = { ...base, offBacklog: { done: 1, canceled: 0 } };
    expect(bulkChangeToast(r, rows, "").text).toBe("2 件を変えました");
    expect(bulkChangeToast(r, rows, offBacklogText(r)).text).toBe(
      "2 件を変えました。1 件は完了のため、バックログには表示されません",
    );
  });
});

describe("off backlog", () => {
  const o = (done: number, canceled: number) =>
    offBacklogText({ ...base, offBacklog: { done, canceled } });
  it("texts", () => {
    expect(o(2, 0)).toBe("。2 件は完了のため、バックログには表示されません");
    expect(o(0, 1)).toBe("。1 件は中止のため、バックログには表示されません");
    expect(o(1, 1)).toBe("。2 件は完了・中止のため、バックログには表示されません");
    expect(o(0, 0)).toBe("");
  });
});
