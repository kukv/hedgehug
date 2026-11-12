import { describe, expect, it } from "vitest";
import type { AccountRow, SignInCode, SignInTarget } from "../../model/account";
import { get } from "../../reactive/store";
import { code, fakeAccounts, fakeToasts, row } from "./fixtures.test-util";
import { createAccountForm } from "./form";

function setup(rows: AccountRow[] = []) {
  const f = fakeAccounts(rows);
  const { toasts, texts } = fakeToasts();
  const opened: { target: SignInTarget; code: SignInCode }[] = [];
  let saved = 0;
  const form = createAccountForm({
    accounts: f.port,
    toasts,
    signIn: (target, c) => void opened.push({ target, code: c }),
    onSaved: () => {
      saved += 1;
    },
  });
  return { f, texts, opened, form, saved: () => saved };
}

const clientIdText =
  "デバイスフローを使うには、config.yaml の hosts に github.com の client_id を設定してください。gh CLI のトークン・環境変数・OS のキーチェーンは、設定なしで使えます";

describe("createAccountForm", () => {
  it("opens add with the device flow first and empty fields", () => {
    const { form } = setup();
    form.openAdd();
    expect(get(form.state)).toEqual({
      original: null,
      form: { name: "", login: "", method: "device" },
      error: "",
      loginError: "",
      asking: false,
      confirm: null,
    });
  });

  it("adds a gh account after GitHub confirms it", async () => {
    const { f, form, texts, saved } = setup();
    form.openAdd();
    form.update({ name: "work", login: "taro", method: "gh" });
    await form.submit();
    expect(f.calls).toEqual(["add work taro gh"]);
    expect(get(form.state)).toBeNull();
    expect(texts()).toEqual(["アカウントを追加しました"]);
    expect(saved()).toBe(1);
  });

  it("shows the refusal and keeps the input", async () => {
    const { f, form } = setup();
    form.openAdd();
    form.update({ name: " ", login: "taro", method: "env" });
    f.refuse.add = "表示名とログイン名を入力してください";
    await form.submit();
    expect(get(form.state)).toMatchObject({
      error: "表示名とログイン名を入力してください",
      asking: false,
      form: { name: " ", login: "taro", method: "env" },
    });
  });

  it("does not send twice while asking", async () => {
    const { f, form } = setup();
    form.openAdd();
    form.update({ name: "work", login: "taro", method: "gh" });
    await Promise.all([form.submit(), form.submit()]);
    expect(f.calls).toEqual(["add work taro gh"]);
  });

  it("gets a code and hands it to the sign-in for the device flow", async () => {
    const { f, form, opened } = setup();
    form.openAdd();
    form.update({ name: "work", login: "taro" });
    await form.submit();
    expect(f.calls).toEqual(["startSignIn - work taro"]);
    expect(get(form.state)).toBeNull();
    expect(opened).toEqual([
      { target: { purpose: "add", original: null, name: "work", login: "taro" }, code: code("S1") },
    ]);
  });

  it("shows why the code could not be had, in the dialog", async () => {
    const { f, form, opened } = setup();
    form.openAdd();
    form.update({ name: "work", login: "taro" });
    f.refuse.startSignIn = clientIdText;
    await form.submit();
    expect(get(form.state)?.error).toBe(clientIdText);
    expect(opened).toEqual([]);
  });

  it("cancels the code when the dialog was closed while asking", async () => {
    const { f, form, opened } = setup();
    form.openAdd();
    form.update({ name: "work", login: "taro" });
    const sending = form.submit();
    form.close();
    await sending;
    expect(f.calls).toEqual(["startSignIn - work taro", "cancelSignIn S1"]);
    expect(opened).toEqual([]);
  });

  it("edits with the current values, and asks first when あなた changes", async () => {
    const a = row("A1", { name: "work", login: "taro" });
    const { f, form, texts } = setup([a]);
    form.openEdit(a);
    expect(get(form.state)?.form).toEqual({ name: "work", login: "taro", method: "gh" });
    expect(get(form.state)?.original).toEqual({
      id: "A1",
      name: "work",
      login: "taro",
      method: "gh",
    });
    form.update({ login: "jiro" });
    f.impacts.edit = ["チーム での あなた が @taro から jiro に変わります。"];
    await form.submit();
    expect(get(form.state)?.confirm).toEqual([
      "チーム での あなた が @taro から jiro に変わります。",
    ]);
    expect(f.calls).toEqual(["editImpact A1 jiro"]);
    form.cancelConfirm();
    expect(get(form.state)?.confirm).toBeNull();
    await form.submit();
    await form.confirm();
    expect(f.calls).toEqual(["editImpact A1 jiro", "editImpact A1 jiro", "edit A1 work jiro gh"]);
    expect(get(form.state)).toBeNull();
    expect(texts()).toEqual(["アカウントを変更しました"]);
  });

  it("saves without asking when the login stays or あなた does not change", async () => {
    const a = row("A1", { name: "work", login: "taro" });
    const { f, form } = setup([a]);
    form.openEdit(a);
    form.update({ name: "home" });
    await form.submit();
    form.openEdit(a);
    form.update({ login: "jiro" });
    await form.submit();
    expect(f.calls).toEqual(["edit A1 home taro gh", "editImpact A1 jiro", "edit A1 home jiro gh"]);
  });

  it("signs in again with the account when the method becomes the device flow", async () => {
    const a = row("A1", { name: "work", login: "taro" });
    const { f, form, opened } = setup([a]);
    form.openEdit(a);
    form.update({ method: "device" });
    await form.submit();
    expect(f.calls).toEqual(["startSignIn A1 work taro"]);
    expect(opened[0]?.target).toEqual({
      purpose: "edit",
      original: { id: "A1", name: "work", login: "taro", method: "gh" },
      name: "work",
      login: "taro",
    });
  });

  it("comes back from the sign-in with the device flow and the red text", () => {
    const { form } = setup();
    form.back(
      { purpose: "add", original: null, name: "work", login: "taro" },
      { loginError: "このトークンは @jiro のものです。ログイン名を直してください" },
    );
    expect(get(form.state)).toMatchObject({
      original: null,
      form: { name: "work", login: "taro", method: "device" },
      loginError: "このトークンは @jiro のものです。ログイン名を直してください",
      error: "",
    });
  });
});
