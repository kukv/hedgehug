import { describe, expect, it } from "vitest";
import { goneText, movedLine } from "./movedText";

const now = new Date(2026, 10, 14, 12, 0);

describe("goneText (02「見つからないとき」の 4 つの文)", () => {
  const at = new Date(2026, 10, 12, 9, 0).toISOString(); // 今年なので 11/12 09:00
  it.each([
    [
      { reason: "deleted", newDisplay: "MB-2" },
      "HH-2 は 11/12 09:00 に MB-2 へ移動しましたが、削除されています。設定の「削除済み」から復元できます",
    ],
    [
      { reason: "purged", newDisplay: "TM-2" },
      "HH-2 は 11/12 09:00 に TM-2 へ移動しましたが、完全に削除されました",
    ],
    [
      { reason: "hidden", newDisplay: "WEB-31", storeName: "elsewhere" },
      "HH-2 は 11/12 09:00 に elsewhere へ移動しました。この Store はいま表示できません",
    ],
    [
      { reason: "elsewhere", newDisplay: "WEB-31", storeName: "elsewhere" },
      "HH-2 は 11/12 09:00 に WEB-31 へ移動し、その後 elsewhere へ移動しました。この Store はいま表示できません",
    ],
  ] as const)("%j", (g, want) => {
    expect(goneText({ ...g, old: "HH-2", at }, now)).toBe(want);
  });

  it("writes the year when the move was in another year", () => {
    const last = new Date(2025, 11, 31, 23, 59).toISOString();
    expect(goneText({ reason: "purged", old: "HH-2", at: last, newDisplay: "TM-2" }, now)).toBe(
      "HH-2 は 2025/12/31 23:59 に TM-2 へ移動しましたが、完全に削除されました",
    );
  });
});

describe("movedLine (02「移した日時の表示」)", () => {
  const at = new Date(2026, 10, 13, 9, 5).toISOString();
  it("says one move when the item has not moved since", () => {
    expect(movedLine({ old: "HH-5", at, display: "WEB-1", current: "WEB-1" }, now)).toBe(
      "HH-5 は 11/13 09:05 に WEB-1 へ移動しました",
    );
  });
  it("says the later move too when the item moved again (A → B → C)", () => {
    expect(movedLine({ old: "HH-5", at, display: "TM-4", current: "AR-7" }, now)).toBe(
      "HH-5 は 11/13 09:05 に TM-4 へ移動し、その後 AR-7 へ移動しました",
    );
  });
});
