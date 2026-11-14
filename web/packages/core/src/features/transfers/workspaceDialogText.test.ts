import { describe, expect, it } from "vitest";
import type { WorkspaceTransferPreview } from "../../model/transfer";
import {
  noWorkspaceTargetsHiddenMessage,
  noWorkspaceTargetsMessage,
  workspaceMovedMessage,
  workspaceTransferLines,
  workspaceTransferNote,
} from "./workspaceDialogText";

const preview: WorkspaceTransferPreview = {
  source: "e2e",
  destination: "team",
  projects: 2,
  items: 5,
  deleted: 0,
  people: [],
  remoteNotice: false,
  destinationHasRemote: false,
};
const outside =
  "ワークスペースの外にあるアイテムとの関連は、種類を問わず解除されます。ワークスペースの中のアイテム同士の関連は残ります。";

describe("workspaceTransferLines (03「別の Store へ移動」の確認ダイアログの点)", () => {
  it("writes the move, the counts and the outside relations in order", () => {
    expect(workspaceTransferLines("Web", preview)).toEqual([
      "「Web」を e2e から team へ移動します。",
      "プロジェクト 2 件・アイテム 5 件を移動します。",
      outside,
    ]);
  });

  it("adds the deleted count to the second sentence", () => {
    expect(workspaceTransferLines("Web", { ...preview, deleted: 3 })[1]).toBe(
      "プロジェクト 2 件・アイテム 5 件を移動します。削除済みの 3 件も一緒に移ります。",
    );
  });

  it("names the people to add, and omits the line for none", () => {
    const people = [
      { login: "suzuki", name: "鈴木" },
      { login: "takahashi", name: "高橋" },
    ];
    expect(workspaceTransferLines("Web", { ...preview, people })[3]).toBe(
      "鈴木さん・高橋さんの 2 人が、移動先のメンバーに追加されます",
    );
    expect(workspaceTransferLines("Web", preview)).toHaveLength(3);
  });

  it("adds the remote notice last, only when asked", () => {
    const people = [{ login: "suzuki", name: "鈴木" }];
    const lines = workspaceTransferLines("Web", { ...preview, people, remoteNotice: true });
    expect(lines).toHaveLength(5);
    expect(lines[4]).toBe("移動先に remote がないため、ほかの人の Store からは見えなくなります。");
  });
});

describe("workspace transfer texts", () => {
  it("uses the spec's wording", () => {
    expect(workspaceTransferNote).toBe(
      "ワークスペースをまるごと、別の git リポジトリへ移動します。移動先には、この Store の履歴は引き継がれません。",
    );
    expect(noWorkspaceTargetsMessage).toBe("移せる Store がありません");
    expect(noWorkspaceTargetsHiddenMessage).toBe(
      "移せる Store がありません(非表示の Store は、設定の「Store」で表示に戻してください)",
    );
    expect(workspaceMovedMessage("team")).toBe("team へ移動しました");
  });
});
