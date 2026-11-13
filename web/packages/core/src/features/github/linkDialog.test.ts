import { describe, expect, it } from "vitest";
import { NotFound, Refused } from "../../model/errors";
import { get } from "../../reactive/store";
import { fakeLinks } from "./fixtures.test-util";
import { createLinkDialog } from "./linkDialog";

function setup() {
  const links = fakeLinks();
  let added = 0;
  const dialog = createLinkDialog({ links: links.port, onAdded: () => (added += 1) });
  return { links, dialog, added: () => added };
}

describe("createLinkDialog", () => {
  it("picks the first project repository and does not add it again", async () => {
    const t = setup();
    t.dialog.open("pr", "S1", "I1", ["org/web", "org/api"]);
    expect(get(t.dialog.state)).toMatchObject({
      kind: "pr",
      repo: "org/web",
      number: "",
      addToProject: true,
      asking: false,
      error: "",
    });
    t.dialog.update({ number: "41" });
    await t.dialog.submit();
    expect(t.links.calls).toEqual(["add S1 I1 pr org/web 41 false"]);
    expect(get(t.dialog.state)).toBeNull();
    expect(t.added()).toBe(1);
  });

  it("sends the typed repository and the check when the project has none", async () => {
    const t = setup();
    t.dialog.open("issue", "S1", "I1", []);
    t.dialog.update({ repo: "org/api", number: "12" });
    await t.dialog.submit();
    expect(t.links.calls).toEqual(["add S1 I1 issue org/api 12 true"]);
  });

  it("takes repositories that arrive after opening, keeping a typed one only if it is a choice", () => {
    const t = setup();
    t.dialog.open("pr", "S1", "I1", []);
    t.dialog.setRepos(["org/web"]);
    expect(get(t.dialog.state)).toMatchObject({ repos: ["org/web"], repo: "org/web" });
    t.dialog.open("pr", "S1", "I1", []);
    t.dialog.update({ repo: "org/api" }); // 選択肢にない: 先頭を選ぶ
    t.dialog.setRepos(["org/web"]);
    expect(get(t.dialog.state)).toMatchObject({ repos: ["org/web"], repo: "org/web" });
    t.dialog.open("pr", "S1", "I1", []);
    t.dialog.update({ repo: "ORG/API" }); // 大文字小文字だけ違う: その選択肢を選ぶ
    t.dialog.setRepos(["org/web", "org/api"]);
    expect(get(t.dialog.state)).toMatchObject({ repo: "org/api" });
  });

  it("sends false when the add-to-project check is cleared", async () => {
    const t = setup();
    t.dialog.open("pr", "S1", "I1", []);
    t.dialog.update({ repo: "org/web", number: "41", addToProject: false });
    await t.dialog.submit();
    expect(t.links.calls).toEqual(["add S1 I1 pr org/web 41 false"]);
  });

  it("does not let an old answer close a dialog opened again", async () => {
    const t = setup();
    let release: () => void = () => undefined;
    t.links.port.add = () =>
      new Promise<void>((resolve) => {
        release = resolve;
      });
    t.dialog.open("pr", "S1", "I1", ["org/web"]);
    t.dialog.update({ number: "41" });
    const first = t.dialog.submit();
    t.dialog.close();
    t.dialog.open("issue", "S1", "I1", ["org/web"]);
    release();
    await first;
    expect(get(t.dialog.state)).toMatchObject({ kind: "issue", asking: false });
    expect(t.added()).toBe(0);
  });

  it("ignores repositories while closed", () => {
    const t = setup();
    t.dialog.setRepos(["org/web"]);
    expect(get(t.dialog.state)).toBeNull();
  });

  it("keeps the dialog open with the refusal", async () => {
    const t = setup();
    t.dialog.open("pr", "S1", "I1", []);
    t.links.fail = new Refused("番号を入力してください");
    await t.dialog.submit();
    expect(get(t.dialog.state)).toMatchObject({ asking: false, error: "番号を入力してください" });
    expect(t.added()).toBe(0);
  });

  it("asks once at a time and drops the answer after closing", async () => {
    const t = setup();
    t.dialog.open("pr", "S1", "I1", ["org/web"]);
    t.dialog.update({ number: "41" });
    const first = t.dialog.submit();
    expect(get(t.dialog.state)?.asking).toBe(true);
    await t.dialog.submit(); // 問い合わせ中: 送らない
    t.dialog.close();
    await first;
    expect(t.links.calls).toHaveLength(1);
    expect(get(t.dialog.state)).toBeNull();
  });

  it("closes when the item is gone and keeps quiet on other failures", async () => {
    const t = setup();
    t.dialog.open("pr", "S1", "I1", ["org/web"]);
    t.links.fail = new Error("network");
    await t.dialog.submit();
    expect(get(t.dialog.state)).toMatchObject({ asking: false, error: "" });
    t.links.fail = new NotFound();
    await t.dialog.submit();
    expect(get(t.dialog.state)).toBeNull();
  });
});
