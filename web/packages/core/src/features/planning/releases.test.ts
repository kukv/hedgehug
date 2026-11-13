import { describe, expect, it } from "vitest";
import type { Release, ReleaseCard } from "../../model/planning";
import {
  dateText,
  deleteReleaseBody,
  newReleaseForm,
  overdue,
  progressRatio,
  progressText,
  releaseConfirm,
  releaseCreated,
  releaseDeleted,
  releaseForm,
  releaseMenu,
  releaseStateChanged,
  stateColor,
  stateLabel,
  stateMenu,
} from "./releases";

const release = (over: Partial<Release> = {}): Release => ({
  id: "R1",
  name: "v2.1.0",
  date: "2026-11-20",
  description: "",
  state: "unreleased",
  ...over,
});

const card = (over: Partial<ReleaseCard> = {}, r: Partial<Release> = {}): ReleaseCard => ({
  release: release(r),
  target: 0,
  done: 0,
  undone: 0,
  items: [],
  ...over,
});

describe("release states", () => {
  it("names the states and their pill colors", () => {
    expect(stateLabel("unreleased")).toBe("未リリース");
    expect(stateLabel("released")).toBe("リリース済み");
    expect(stateLabel("archived")).toBe("アーカイブ");
    expect(stateColor("unreleased")).toBe("blue");
    expect(stateColor("released")).toBe("green");
    expect(stateColor("archived")).toBe("gray");
  });

  it("offers the two states other than the current one", () => {
    expect(stateMenu(release())).toEqual([
      { to: "released", label: "リリース済みにする" },
      { to: "archived", label: "アーカイブにする" },
    ]);
    expect(stateMenu(release({ state: "archived" }))).toEqual([
      { to: "unreleased", label: "未リリースにする" },
      { to: "released", label: "リリース済みにする" },
    ]);
  });
});

describe("overdue", () => {
  it("is overdue only when unreleased and the date is before today", () => {
    expect(overdue(release({ date: "2026-11-13" }), "2026-11-14")).toBe(true);
    expect(overdue(release({ date: "2026-11-14" }), "2026-11-14")).toBe(false);
    expect(overdue(release({ date: "2026-11-13", state: "released" }), "2026-11-14")).toBe(false);
  });

  it("is not overdue without a date", () => {
    expect(overdue(release({ date: "" }), "2026-11-14")).toBe(false);
  });
});

describe("dateText", () => {
  it("shows the date, or 日付未定 without one", () => {
    expect(dateText(release(), 2026)).toBe("11/20");
    expect(dateText(release({ date: "" }), 2026)).toBe("日付未定");
  });
});

describe("progress", () => {
  it("adds the rest only while some remain", () => {
    expect(progressText(card({ target: 5, done: 3, undone: 2 }))).toBe("3 / 5 件完了 · 残り 2 件");
    expect(progressText(card({ target: 5, done: 5 }))).toBe("5 / 5 件完了");
    expect(progressText(card())).toBe("0 / 0 件完了");
  });

  it("is the ratio of done to the target, or 0 without a target", () => {
    expect(progressRatio(card({ target: 4, done: 1, undone: 3 }))).toBe(0.25);
    expect(progressRatio(card())).toBe(0);
  });
});

describe("releaseConfirm", () => {
  it("asks when undone items remain", () => {
    expect(releaseConfirm(card({ target: 3, done: 1, undone: 2 }))).toBe(
      "v2.1.0 には未完了のアイテムが 2 件あります。このままリリース済みにしますか?",
    );
  });

  it("does not ask when none remain", () => {
    expect(releaseConfirm(card({ target: 3, done: 3 }))).toBeNull();
  });
});

describe("releaseMenu", () => {
  it("has 編集…, the two states and 削除…", () => {
    expect(releaseMenu(release(), false)).toEqual([
      { id: "edit", label: "編集…", disabled: false },
      { id: "released", label: "リリース済みにする", disabled: false },
      { id: "archived", label: "アーカイブにする", disabled: false },
      { id: "delete", label: "削除…", disabled: false },
    ]);
  });

  it("disables only the state entries while the state is being saved", () => {
    expect(releaseMenu(release(), false, true)).toEqual([
      { id: "edit", label: "編集…", disabled: false },
      { id: "released", label: "リリース済みにする", disabled: true },
      { id: "archived", label: "アーカイブにする", disabled: true },
      { id: "delete", label: "削除…", disabled: false },
    ]);
  });

  it("disables every entry in a read-only Store", () => {
    const menu = releaseMenu(release(), true);
    expect(menu.every((e) => e.disabled && e.hint === "閲覧のみの Store です")).toBe(true);
  });
});

describe("release dialogs", () => {
  it("starts the create form empty and the edit form from the release", () => {
    expect(newReleaseForm()).toEqual({ name: "", date: "", description: "" });
    expect(releaseForm(release({ description: "初回" }))).toEqual({
      name: "v2.1.0",
      date: "2026-11-20",
      description: "初回",
    });
  });

  it("writes the toasts", () => {
    expect(releaseCreated("v2.1.0")).toBe("v2.1.0 を作成しました");
    expect(releaseStateChanged("v2.1.0", "released")).toBe("v2.1.0 をリリース済みにしました");
    expect(releaseDeleted).toBe("リリースを削除しました");
  });

  it("tells how many items lose the release, and how long it can be restored", () => {
    expect(deleteReleaseBody("v2.1.0", 3, 30)).toBe(
      "「v2.1.0」を削除します。3 件のアイテムからリリースが解除されます。設定の「削除済み」から戻せます。30 日を過ぎると完全に削除されます。",
    );
    expect(deleteReleaseBody("v2.1.0", 0, 30)).toBe(
      "「v2.1.0」を削除します。設定の「削除済み」から戻せます。30 日を過ぎると完全に削除されます。",
    );
  });
});
