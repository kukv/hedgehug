import { cleanup, fireEvent } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Account } from "../../core/accounts";
import { renderWithCore } from "../test-utils";
import { resetToasts, toasts } from "../toast.svelte";
import CreateStore from "./CreateStore.svelte";

const created = {
  name: "me",
  path: "/home/u/.local/share/hedgehug/stores/me",
  workspace: {
    ulid: "01HWSME",
    name: "me",
    id_prefix: "ME",
    prefix_set: "2026-11-02T00:00:00Z",
    tasks: 0,
  },
  remotes: [],
  pushed: [],
};

const acct = (name: string): Account => ({
  name,
  id: `github.com/${name}`,
  state: "ok",
  login: name,
  token: "keychain",
});

function mount(
  post = vi.fn().mockResolvedValue(created),
  accts: { list?: Account[]; loaded?: boolean } = {},
) {
  const reload = vi.fn().mockResolvedValue(undefined);
  const onclose = vi.fn();
  const oncreated = vi.fn();
  const accounts = {
    accounts: () => accts.list ?? [],
    loaded: () => accts.loaded ?? true,
    error: () => null,
    load: vi.fn(async () => {}),
    reload: vi.fn(async () => {}),
    subscribe: () => () => {},
  };
  const r = renderWithCore(
    CreateStore,
    { open: true, onclose, oncreated },
    { api: { initStore: post }, stores: { reload, subscribe: () => () => {} }, accounts },
  );
  return { post, reload, onclose, oncreated, accounts, ...r };
}

const type = (el: HTMLElement, value: string) => fireEvent.input(el, { target: { value } });

beforeEach(() => {
  history.replaceState(null, "", "/");
  resetToasts();
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
    configurable: true,
  });
});
afterEach(cleanup);

describe("CreateStore", () => {
  it("名前と接頭辞が空なら送らず、理由を出す", async () => {
    const { getByRole, getByText, post } = mount();
    await fireEvent.click(getByRole("button", { name: "Store を作成" }));
    expect(post).not.toHaveBeenCalled();
    expect(getByText("名前を入れてください")).toBeTruthy();
    expect(getByText("接頭辞は英大文字と数字の 2〜10 文字で、英字から始めてください")).toBeTruthy();
  });

  it("接頭辞は大文字にそろえ、形が違えば送らない", async () => {
    const { getByLabelText, getByRole, post } = mount();
    const prefix = getByLabelText("ID の接頭辞") as HTMLInputElement;
    await type(prefix, "me");
    expect(prefix.value).toBe("ME");
    await type(getByLabelText("名前"), "me");
    await type(prefix, "1A");
    await fireEvent.click(getByRole("button", { name: "Store を作成" }));
    expect(post).not.toHaveBeenCalled();
  });

  it("作成: Store を作成 API、reload、トースト、最初のワークスペースのツリーへ", async () => {
    const { getByLabelText, getByRole, post, reload, onclose } = mount();
    await type(getByLabelText("名前"), " me ");
    await type(getByLabelText("ID の接頭辞"), "me");
    await fireEvent.click(getByRole("button", { name: "Store を作成" }));
    await vi.waitFor(() => expect(onclose).toHaveBeenCalledTimes(1));
    expect(post).toHaveBeenCalledWith({
      name: "me",
      prefix: "ME",
      workspace: undefined,
      remotes: undefined,
    });
    expect(reload).toHaveBeenCalledTimes(1);
    expect(toasts.list.map((t) => t.message)).toEqual(["Store「me」を作成しました"]);
    expect(location.pathname).toBe("/workspaces/me/01HWSME/tree");
  });

  it("最初のワークスペースと remote を渡す", async () => {
    const { getByLabelText, getByRole, post } = mount();
    await type(getByLabelText("名前"), "team");
    await type(getByLabelText("ID の接頭辞"), "TM");
    await type(getByLabelText("最初のワークスペース（任意）"), "backend");
    await type(getByLabelText("remote（任意）"), "git@github.com:org/hh-team.git");
    await fireEvent.click(getByRole("button", { name: "Store を作成" }));
    await vi.waitFor(() => expect(post).toHaveBeenCalledTimes(1));
    expect(post).toHaveBeenCalledWith({
      name: "team",
      prefix: "TM",
      workspace: "backend",
      remotes: ["git@github.com:org/hh-team.git"],
    });
  });

  it("Enter（入力欄の中）でも作成する", async () => {
    const { getByLabelText, post } = mount();
    await type(getByLabelText("名前"), "me");
    const prefix = getByLabelText("ID の接頭辞");
    await type(prefix, "me");
    await fireEvent.keyDown(prefix, { key: "Enter" });
    await vi.waitFor(() => expect(post).toHaveBeenCalledTimes(1));
  });

  it("失敗したらエラーの文言を出し、ダイアログは閉じない", async () => {
    const post = vi.fn().mockRejectedValue(new Error("boom"));
    const { getByLabelText, getByRole, onclose } = mount(post);
    await type(getByLabelText("名前"), "me");
    await type(getByLabelText("ID の接頭辞"), "ME");
    await fireEvent.click(getByRole("button", { name: "Store を作成" }));
    const alert = await vi.waitFor(() => getByRole("alert"));
    expect(alert.textContent).toContain("操作に失敗しました: boom");
    expect(onclose).not.toHaveBeenCalled();
    expect((getByRole("button", { name: "Store を作成" }) as HTMLButtonElement).disabled).toBe(
      false,
    );
  });

  it("足元に同じ操作の hh コマンドを出す", async () => {
    const { getByLabelText, getByRole } = mount();
    await type(getByLabelText("名前"), "me");
    await type(getByLabelText("ID の接頭辞"), "me");
    expect(getByRole("button", { name: /hh store init me/ }).textContent).toContain("ME");
  });

  it("入力がそろうまでは、足元の hh は <名前> の形でコピーできない", async () => {
    const { getByLabelText, getByRole } = mount();
    const line = () => getByRole("button", { name: /hh store init/ }) as HTMLButtonElement;
    expect(line().textContent).toContain("hh store init <名前> --prefix <接頭辞>");
    expect(line().disabled).toBe(true);
    await type(getByLabelText("名前"), "me");
    expect(line().disabled).toBe(true);
    await type(getByLabelText("ID の接頭辞"), "m");
    expect(line().disabled).toBe(true);
    await type(getByLabelText("ID の接頭辞"), "me");
    expect(line().disabled).toBe(false);
  });

  it("変換中（isComposing）の Enter では作成しない", async () => {
    const { getByLabelText, post } = mount();
    await type(getByLabelText("名前"), "me");
    const prefix = getByLabelText("ID の接頭辞");
    await type(prefix, "me");
    await fireEvent.keyDown(prefix, { key: "Enter", isComposing: true });
    expect(post).not.toHaveBeenCalled();
  });

  it("作成できたあとに stores.reload が失敗しても、作成の成功として扱う", async () => {
    const post = vi.fn().mockResolvedValue(created);
    const reload = vi.fn().mockRejectedValue(new Error("network"));
    const onclose = vi.fn();
    const { getByLabelText, getByRole, queryByRole } = renderWithCore(
      CreateStore,
      { open: true, onclose },
      { api: { initStore: post }, stores: { reload, subscribe: () => () => {} } },
    );
    await type(getByLabelText("名前"), "me");
    await type(getByLabelText("ID の接頭辞"), "me");
    await fireEvent.click(getByRole("button", { name: "Store を作成" }));
    await vi.waitFor(() => expect(onclose).toHaveBeenCalledTimes(1));
    expect(reload).toHaveBeenCalledTimes(1);
    expect(queryByRole("alert")).toBeNull();
    expect(toasts.list.map((t) => t.message)).toEqual(["Store「me」を作成しました"]);
    expect(location.pathname).toBe("/workspaces/me/01HWSME/tree");
  });

  it("キャンセルすると失敗の表示を消す", async () => {
    const post = vi.fn().mockRejectedValue(new Error("boom"));
    const { getByLabelText, getByRole, queryByRole, onclose } = mount(post);
    await type(getByLabelText("名前"), "me");
    await type(getByLabelText("ID の接頭辞"), "ME");
    await fireEvent.click(getByRole("button", { name: "Store を作成" }));
    await vi.waitFor(() => getByRole("alert"));
    await fireEvent.click(getByRole("button", { name: "キャンセル" }));
    expect(onclose).toHaveBeenCalledTimes(1);
    expect(queryByRole("alert")).toBeNull();
  });

  it("作成中はキャンセルできず、閉じない", async () => {
    let finish: (v: unknown) => void = () => {};
    const post = vi.fn().mockReturnValue(new Promise((r) => (finish = r)));
    const { getByLabelText, getByRole, onclose } = mount(post);
    await type(getByLabelText("名前"), "me");
    await type(getByLabelText("ID の接頭辞"), "ME");
    await fireEvent.click(getByRole("button", { name: "Store を作成" }));
    const cancel = getByRole("button", { name: "キャンセル" }) as HTMLButtonElement;
    expect(cancel.disabled).toBe(true);
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(onclose).not.toHaveBeenCalled();
    finish(created);
    await vi.waitFor(() => expect(onclose).toHaveBeenCalledTimes(1));
  });

  it("作成できたら oncreated を呼ぶ", async () => {
    const { getByLabelText, getByRole, oncreated } = mount();
    await type(getByLabelText("名前"), "me");
    await type(getByLabelText("ID の接頭辞"), "me");
    await fireEvent.click(getByRole("button", { name: "Store を作成" }));
    await vi.waitFor(() => expect(oncreated).toHaveBeenCalledTimes(1));
  });

  it("失敗したら oncreated を呼ばない", async () => {
    const post = vi.fn().mockRejectedValue(new Error("boom"));
    const { getByLabelText, getByRole, oncreated } = mount(post);
    await type(getByLabelText("名前"), "me");
    await type(getByLabelText("ID の接頭辞"), "ME");
    await fireEvent.click(getByRole("button", { name: "Store を作成" }));
    await vi.waitFor(() => getByRole("alert"));
    expect(oncreated).not.toHaveBeenCalled();
  });
});

describe("CreateStore のアカウント", () => {
  const fill = async (r: ReturnType<typeof mount>) => {
    await type(r.getByLabelText("名前"), "me");
    await type(r.getByLabelText("ID の接頭辞"), "me");
  };

  it("開いたときにアカウントを読む", () => {
    const { accounts } = mount();
    expect(accounts.load).toHaveBeenCalled();
  });

  it("アカウントが 0 件か 1 件なら選択欄を出さず、account を送らない", async () => {
    for (const list of [[], [acct("a")]]) {
      const r = mount(undefined, { list });
      expect(r.queryByLabelText("アカウント")).toBeNull();
      await fill(r);
      await fireEvent.click(r.getByRole("button", { name: "Store を作成" }));
      await vi.waitFor(() => expect(r.post).toHaveBeenCalledTimes(1));
      expect(r.post.mock.calls[0]?.[0].account).toBeUndefined();
      expect(r.getByRole("button", { name: /hh store init/ }).textContent).not.toContain(
        "--account",
      );
      cleanup();
    }
  });

  it("読み込み中は選択欄を出さず、account を送らない", async () => {
    const r = mount(undefined, { list: [acct("a"), acct("b")], loaded: false });
    expect(r.queryByLabelText("アカウント")).toBeNull();
    await fill(r);
    await fireEvent.click(r.getByRole("button", { name: "Store を作成" }));
    await vi.waitFor(() => expect(r.post).toHaveBeenCalledTimes(1));
    expect(r.post.mock.calls[0]?.[0].account).toBeUndefined();
  });

  it("2 つ以上なら選択欄を出し、選ばないと送らない", async () => {
    const r = mount(undefined, { list: [acct("a"), acct("b")] });
    const pick = r.getByLabelText("アカウント") as HTMLSelectElement;
    expect(Array.from(pick.options).map((o) => o.value)).toEqual(["", "a", "b"]);
    expect(pick.value).toBe("");
    await fill(r);
    await fireEvent.click(r.getByRole("button", { name: "Store を作成" }));
    expect(r.getByText("アカウントを選んでください")).toBeTruthy();
    expect(r.post).not.toHaveBeenCalled();
  });

  it("選んだアカウントを送り、足元に --account を出す", async () => {
    const r = mount(undefined, { list: [acct("a"), acct("b")] });
    await fill(r);
    await fireEvent.change(r.getByLabelText("アカウント"), { target: { value: "b" } });
    expect(r.getByRole("button", { name: /hh store init/ }).textContent).toContain("--account b");
    await fireEvent.click(r.getByRole("button", { name: "Store を作成" }));
    await vi.waitFor(() => expect(r.post).toHaveBeenCalledTimes(1));
    expect(r.post).toHaveBeenCalledWith({
      name: "me",
      prefix: "ME",
      workspace: undefined,
      remotes: undefined,
      account: "b",
    });
  });
});
