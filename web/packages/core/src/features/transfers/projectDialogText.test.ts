import { describe, expect, it } from "vitest";
import type { ProjectTransferPreview } from "../../model/transfer";
import {
  notCarriedLine,
  projectMovedMessage,
  projectTransferIntro,
  replacementLabel,
  replacementSelectLabel,
} from "./projectDialogText";

const preview: ProjectTransferPreview = {
  items: 3,
  deleted: 0,
  prefix: "MB",
  source: "Web",
  destination: "Mobile",
  crossStore: false,
  remoteNotice: false,
  destinationHasRemote: false,
  hasRules: false,
  notCarried: [],
  replacements: [],
};

describe("projectTransferIntro (02 **移動** の 1・3 つ目の点)", () => {
  it("names the title, the source, the destination, the count and the prefix", () => {
    expect(projectTransferIntro("Web", preview)).toBe(
      "「Web」を Web から Mobile へ移動します。3 件のアイテムは MB の番号になります。",
    );
  });

  it("adds the deleted items", () => {
    expect(projectTransferIntro("Web", { ...preview, deleted: 1 })).toBe(
      "「Web」を Web から Mobile へ移動します。3 件のアイテムは MB の番号になります。削除済みの 1 件も一緒に移ります。",
    );
  });

  it("adds the cross-Store, remote and rules sentences in that order", () => {
    const p = {
      ...preview,
      destination: "team の Team",
      crossStore: true,
      remoteNotice: true,
      hasRules: true,
    };
    expect(projectTransferIntro("Web", p)).toBe(
      "「Web」を Web から team の Team へ移動します。3 件のアイテムは MB の番号になります。" +
        "移さないアイテムとの関連を解除します。移すもの同士の関連は残ります。移動先のメンバーにいない人は、移動先のメンバーに加わります。" +
        "移動先に remote がないため、ほかの人の Store からは見えなくなります。" +
        "このプロジェクトのルールは、移動先の同じ名前のステータス・テンプレートに合わせて移ります。",
    );
  });
});

describe("notCarriedLine (A5b-8)", () => {
  const n = { name: "", missing: [], count: 0 };
  it("writes each kind", () => {
    expect(notCarriedLine({ ...n, kind: "hidden", name: "Pending" })).toBe(
      "使わないステータス「Pending」(移動先にありません)",
    );
    expect(notCarriedLine({ ...n, kind: "hiddenFixed", name: "Todo" })).toBe(
      "使わないステータス「Todo」(移動先では使わないことにできません)",
    );
    expect(
      notCarriedLine({
        ...n,
        kind: "workflow",
        name: "InProgress から Review",
        missing: ["Review"],
      }),
    ).toBe("遷移のチェック「InProgress から Review」(移動先に「Review」がありません)");
    expect(notCarriedLine({ ...n, kind: "template", name: "Bug 報告" })).toBe(
      "使うテンプレート「Bug 報告」(移動先にありません)",
    );
    expect(
      notCarriedLine({ ...n, kind: "templateDefault", name: "Bug", missing: ["Bug 報告"] }),
    ).toBe("Bug の既定のテンプレート「Bug 報告」(移動先にありません)");
    expect(notCarriedLine({ ...n, kind: "disabled", count: 2 })).toBe(
      "ルールを動かさない指定 2 件(移動先のルールには引き継ぎません)",
    );
  });

  it("names both statuses of a transition when neither is in the destination", () => {
    expect(
      notCarriedLine({
        ...n,
        kind: "workflow",
        name: "Review から Pending",
        missing: ["Review", "Pending"],
      }),
    ).toBe("遷移のチェック「Review から Pending」(移動先に「Review」「Pending」がありません)");
  });

  it("writes no line for zero disabled rules", () => {
    expect(notCarriedLine({ ...n, kind: "disabled", count: 0 })).toBeNull();
  });
});

describe("replacement rows and the toast", () => {
  it("shows the count only when items use the status", () => {
    expect(replacementLabel("Review", 2)).toBe("Review(2 件)");
    expect(replacementLabel("Pending", 0)).toBe("Pending");
  });

  it("names the select of a row", () => {
    expect(replacementSelectLabel("Review")).toBe("Review の置き換え先");
  });

  it("toasts the destination", () => {
    expect(projectMovedMessage("Mobile")).toBe("Mobile へ移動しました");
    expect(projectMovedMessage("team の Team")).toBe("team の Team へ移動しました");
  });
});
