import { describe, expect, it } from "vitest";
import {
  changeRows,
  changesHeading,
  noEpicMessage,
  noTargetsMessage,
  offlineMessage,
  replacementsHeading,
  transferFailedMessage,
  transferIntro,
  transferredMessage,
} from "./dialogText";

describe("transferIntro (02「移す先の選び方」)", () => {
  it.each([
    [0, 0, "HH-3 決済 を移動します。"],
    [2, 0, "HH-1 認証 と、その下の 2 件を移動します。"],
    [2, 1, "HH-1 認証 と、その下の 2 件を移動します。削除済みの 1 件も一緒に移ります。"],
    [0, 4, "HH-3 決済 を移動します。削除済みの 4 件も一緒に移ります。"],
  ])("below %d, deleted %d", (below, deleted, want) => {
    const [id, title] = below === 2 ? ["HH-1", "認証"] : ["HH-3", "決済"];
    expect(transferIntro(id, title, below, deleted)).toBe(want);
  });
});

describe("changeRows", () => {
  const p = {
    below: 0,
    deleted: 0,
    nextDisplay: "HH-13",
    crossStore: false,
    remoteNotice: false,
    destinationHasRemote: false,
    replacements: [],
  };
  it("lists the rows for a move inside one Store", () => {
    expect(changeRows(p)).toEqual([
      "ID は HH-13 からの番号に振り直されます",
      "スプリントとリリースは外されます",
      "優先度・関連システムなどは、同じ名前の値に合わせます。合う値がなければ空になります。ステータスは、同じ名前で移動先のプロジェクトが使うものに合わせ、合わないものは「ステータスの置き換え」で選んだステータスにします",
      "移動元に転送の記録(移した日時を含む)を残し、古い ID からも辿れます",
    ]);
  });
  it("adds the cross-Store rows after the value row, and the remote notice only when asked", () => {
    const rows = changeRows({ ...p, crossStore: true });
    expect(rows.slice(3, 5)).toEqual([
      "移さないアイテムとの関連は解除されます。移すもの同士の関連は残ります",
      "移動先のメンバーにいない人は、移動先のメンバーに加わります",
    ]);
    expect(rows).not.toContain(
      "移動先に remote がないため、ほかの人の Store からは見えなくなります。",
    );
    expect(changeRows({ ...p, crossStore: true, remoteNotice: true })[5]).toBe(
      "移動先に remote がないため、ほかの人の Store からは見えなくなります。",
    );
    expect(changeRows({ ...p, crossStore: true, remoteNotice: true }).at(-1)).toBe(
      "移動元に転送の記録(移した日時を含む)を残し、古い ID からも辿れます",
    );
  });
});

describe("messages", () => {
  it("builds the done toast (02「保存」)", () => {
    expect(transferredMessage("HH-5", "MB-2", [])).toBe("HH-5 は MB-2 になりました");
    expect(transferredMessage("HH-5", "MB-2", ["優先度", "関連システム"])).toBe(
      "HH-5 は MB-2 になりました。合う値がなく空になった項目: 優先度・関連システム",
    );
  });
  it("has the fixed texts", () => {
    expect(offlineMessage("team")).toBe("オフラインのため team へ移動できません");
    expect(noTargetsMessage).toBe(
      "移せるワークスペースがありません(プロジェクトのある、書き込めるワークスペースが要ります)",
    );
    expect(noEpicMessage).toBe("このプロジェクトには Epic がありません");
    expect([changesHeading, replacementsHeading, transferFailedMessage]).toEqual([
      "移動先で変わること",
      "ステータスの置き換え",
      "移動できませんでした",
    ]);
  });
});
